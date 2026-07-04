package processor

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/storage"
)

const (
	defaultProcessTimeout = 30 * time.Minute
	maxStderrExcerptBytes = 2048
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) (CommandResult, error)
}

type CommandResult struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

type OSCommandRunner struct{}

func (OSCommandRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	result := CommandResult{
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
		ExitCode: 0,
	}
	if err != nil {
		result.ExitCode = -1
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		}
	}
	return result, err
}

type FFmpegProcessor struct {
	objectStore     storage.ObjectStore
	processedBucket string
	thumbnailBucket string
	ffmpegPath      string
	ffprobePath     string
	timeout         time.Duration
	runner          CommandRunner
	now             func() time.Time
}

type FFmpegConfig struct {
	ObjectStore     storage.ObjectStore
	ProcessedBucket string
	ThumbnailBucket string
	FFmpegPath      string
	FFprobePath     string
	Timeout         time.Duration
	Runner          CommandRunner
	Now             func() time.Time
}

func NewFFmpegProcessor(config FFmpegConfig) (*FFmpegProcessor, error) {
	if config.ObjectStore == nil {
		return nil, fmt.Errorf("object store is required")
	}
	processedBucket := strings.TrimSpace(config.ProcessedBucket)
	if processedBucket == "" {
		return nil, fmt.Errorf("processed bucket is required")
	}
	thumbnailBucket := strings.TrimSpace(config.ThumbnailBucket)
	if thumbnailBucket == "" {
		return nil, fmt.Errorf("thumbnail bucket is required")
	}
	ffmpegPath := strings.TrimSpace(config.FFmpegPath)
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	ffprobePath := strings.TrimSpace(config.FFprobePath)
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultProcessTimeout
	}
	runner := config.Runner
	if runner == nil {
		runner = OSCommandRunner{}
	}
	now := config.Now
	if now == nil {
		now = time.Now
	}
	return &FFmpegProcessor{
		objectStore:     config.ObjectStore,
		processedBucket: processedBucket,
		thumbnailBucket: thumbnailBucket,
		ffmpegPath:      ffmpegPath,
		ffprobePath:     ffprobePath,
		timeout:         timeout,
		runner:          runner,
		now:             now,
	}, nil
}

func (p *FFmpegProcessor) Process(ctx context.Context, job domain.ProcessingJob, attempt domain.ProcessingAttempt) (Result, error) {
	startedAt := p.now().UTC()
	processCtx, cancel := context.WithTimeout(ctx, p.timeout)
	defer cancel()

	tempDir, err := os.MkdirTemp("", "media-worker-"+safePathPart(job.VideoID)+"-*")
	if err != nil {
		return Result{}, processingFailure(domain.CodeUnknownProcessingError, "create processing workspace failed", true)
	}
	defer os.RemoveAll(tempDir)

	inputPath := filepath.Join(tempDir, "input")
	processedPath := filepath.Join(tempDir, "processed.mp4")
	thumbnailPath := filepath.Join(tempDir, "poster.jpg")

	inputBytes, err := p.downloadInput(processCtx, job, inputPath)
	if err != nil {
		return Result{}, err
	}
	probe, err := p.probe(processCtx, inputPath)
	if err != nil {
		return Result{}, err
	}
	if _, err := p.runCommand(processCtx, "ffmpeg transcode", p.ffmpegPath, transcodeArgs(inputPath, processedPath)...); err != nil {
		return Result{}, err
	}
	if _, err := p.runCommand(processCtx, "ffmpeg thumbnail", p.ffmpegPath, thumbnailArgs(inputPath, thumbnailPath)...); err != nil {
		return Result{}, err
	}

	processedInfo, err := os.Stat(processedPath)
	if err != nil {
		return Result{}, processingFailure(domain.CodeFFmpegFailed, "processed output was not created", false)
	}
	if processedInfo.Size() <= 0 {
		return Result{}, processingFailure(domain.CodeFFmpegFailed, "processed output is empty", false)
	}
	if thumbnailInfo, err := os.Stat(thumbnailPath); err != nil || thumbnailInfo.Size() <= 0 {
		return Result{}, processingFailure(domain.CodeFFmpegFailed, "thumbnail output was not created", false)
	}

	processedKey := processedObjectKey(job.VideoID)
	thumbnailKey := thumbnailObjectKey(job.VideoID)
	if err := p.uploadFile(processCtx, p.processedBucket, processedKey, "video/mp4", processedPath); err != nil {
		return Result{}, err
	}
	if err := p.uploadFile(processCtx, p.thumbnailBucket, thumbnailKey, "image/jpeg", thumbnailPath); err != nil {
		return Result{}, err
	}

	metrics, _ := json.Marshal(map[string]any{
		"processor":            "ffmpeg",
		"attempt_no":           attempt.AttemptNo,
		"input_bytes":          inputBytes,
		"output_bytes":         processedInfo.Size(),
		"duration_ms":          probe.DurationMs,
		"width":                probe.Width,
		"height":               probe.Height,
		"processed_bucket":     p.processedBucket,
		"processed_object_key": processedKey,
		"thumbnail_bucket":     p.thumbnailBucket,
		"thumbnail_object_key": thumbnailKey,
		"elapsed_ms":           p.now().UTC().Sub(startedAt).Milliseconds(),
		"generated_at":         p.now().UTC().Format(time.RFC3339Nano),
	})

	return Result{
		ProcessedObjectKey: processedKey,
		ThumbnailObjectKey: thumbnailKey,
		DurationMs:         probe.DurationMs,
		Width:              probe.Width,
		Height:             probe.Height,
		SizeBytes:          processedInfo.Size(),
		Metrics:            metrics,
	}, nil
}

func (p *FFmpegProcessor) downloadInput(ctx context.Context, job domain.ProcessingJob, inputPath string) (int64, error) {
	body, err := p.objectStore.DownloadObject(ctx, storage.ObjectRef{
		Bucket:    job.InputBucket,
		ObjectKey: job.InputObjectKey,
	})
	if err != nil {
		return 0, err
	}
	defer body.Close()

	file, err := os.Create(inputPath)
	if err != nil {
		return 0, processingFailure(domain.CodeUnknownProcessingError, "create local input file failed", true)
	}
	defer file.Close()

	written, err := io.Copy(file, body)
	if err != nil {
		return 0, processingFailure(domain.CodeMinIOUnavailable, "raw object download interrupted", true)
	}
	return written, nil
}

func (p *FFmpegProcessor) probe(ctx context.Context, inputPath string) (probeMetadata, error) {
	result, err := p.runCommand(ctx, "ffprobe metadata", p.ffprobePath,
		"-v", "error",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		inputPath,
	)
	if err != nil {
		return probeMetadata{}, err
	}
	metadata, err := parseProbe(result.Stdout)
	if err != nil {
		return probeMetadata{}, processingFailure(domain.CodeFFmpegFailed, "ffprobe metadata could not be parsed", false)
	}
	return metadata, nil
}

func (p *FFmpegProcessor) uploadFile(ctx context.Context, bucket string, objectKey string, contentType string, path string) error {
	file, err := os.Open(path)
	if err != nil {
		return processingFailure(domain.CodeUnknownProcessingError, "open processed output failed", true)
	}
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return processingFailure(domain.CodeUnknownProcessingError, "stat processed output failed", true)
	}
	_, err = p.objectStore.UploadObject(ctx, storage.UploadObjectInput{
		Bucket:      bucket,
		ObjectKey:   objectKey,
		ContentType: contentType,
		SizeBytes:   info.Size(),
		Body:        file,
	})
	return err
}

func (p *FFmpegProcessor) runCommand(ctx context.Context, label string, name string, args ...string) (CommandResult, error) {
	result, err := p.runner.Run(ctx, name, args...)
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return result, processingFailure(domain.CodeProcessTimeout, label+" timed out", true)
	}
	if err != nil {
		message := label + " failed"
		excerpt := sanitizeStderr(result.Stderr)
		if excerpt != "" {
			message += ": " + excerpt
		}
		return result, processingFailure(domain.CodeFFmpegFailed, message, false)
	}
	return result, nil
}

type probeMetadata struct {
	DurationMs int64
	Width      int
	Height     int
}

func parseProbe(raw []byte) (probeMetadata, error) {
	var payload struct {
		Streams []struct {
			CodecType string `json:"codec_type"`
			Width     int    `json:"width"`
			Height    int    `json:"height"`
		} `json:"streams"`
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return probeMetadata{}, err
	}
	metadata := probeMetadata{}
	if payload.Format.Duration != "" {
		seconds, err := strconv.ParseFloat(payload.Format.Duration, 64)
		if err == nil && seconds > 0 {
			metadata.DurationMs = int64(seconds * 1000)
		}
	}
	for _, stream := range payload.Streams {
		if stream.CodecType == "video" {
			metadata.Width = stream.Width
			metadata.Height = stream.Height
			break
		}
	}
	return metadata, nil
}

func transcodeArgs(inputPath string, outputPath string) []string {
	return []string{
		"-y",
		"-i", inputPath,
		"-map", "0:v:0",
		"-map", "0:a?",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-movflags", "+faststart",
		"-c:a", "aac",
		"-b:a", "128k",
		outputPath,
	}
}

func thumbnailArgs(inputPath string, outputPath string) []string {
	return []string{
		"-y",
		"-ss", "00:00:01",
		"-i", inputPath,
		"-frames:v", "1",
		"-q:v", "2",
		outputPath,
	}
}

func processedObjectKey(videoID string) string {
	return "processed/" + safePathPart(videoID) + "/source.mp4"
}

func thumbnailObjectKey(videoID string) string {
	return "thumbnails/" + safePathPart(videoID) + "/poster.jpg"
}

func safePathPart(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var builder strings.Builder
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
			builder.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			builder.WriteRune(r)
		case r >= '0' && r <= '9':
			builder.WriteRune(r)
		case r == '-', r == '_':
			builder.WriteRune(r)
		default:
			builder.WriteRune('_')
		}
	}
	return builder.String()
}

func sanitizeStderr(raw []byte) string {
	value := strings.TrimSpace(strings.ReplaceAll(string(raw), "\r", "\n"))
	value = strings.Join(strings.Fields(value), " ")
	if len(value) <= maxStderrExcerptBytes {
		return value
	}
	return value[:maxStderrExcerptBytes] + "...truncated"
}

func processingFailure(code string, message string, retryable bool) domain.ProcessingError {
	return domain.ProcessingError{
		Code:      code,
		Message:   message,
		Retryable: retryable,
	}
}
