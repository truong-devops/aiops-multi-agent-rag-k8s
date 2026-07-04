package processor

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/storage"
)

func TestFFmpegProcessorProcessesAndUploadsOutputs(t *testing.T) {
	store := &fakeObjectStore{downloadBody: []byte("raw-video")}
	runner := &fakeRunner{
		onRun: func(name string, args []string) (CommandResult, error) {
			if name == "ffprobe" {
				return CommandResult{Stdout: []byte(`{
					"streams":[{"codec_type":"video","width":1280,"height":720}],
					"format":{"duration":"12.340000"}
				}`)}, nil
			}
			outputPath := args[len(args)-1]
			switch filepath.Base(outputPath) {
			case "processed.mp4":
				if !containsArg(args, "+faststart") {
					t.Fatalf("transcode args missing +faststart: %#v", args)
				}
				if err := os.WriteFile(outputPath, []byte("processed-video"), 0600); err != nil {
					t.Fatalf("write processed output: %v", err)
				}
			case "poster.jpg":
				if err := os.WriteFile(outputPath, []byte("thumbnail"), 0600); err != nil {
					t.Fatalf("write thumbnail output: %v", err)
				}
			default:
				t.Fatalf("unexpected ffmpeg output path %q", outputPath)
			}
			return CommandResult{}, nil
		},
	}
	processor, err := NewFFmpegProcessor(FFmpegConfig{
		ObjectStore:     store,
		ProcessedBucket: "processed-videos",
		ThumbnailBucket: "thumbnails",
		FFmpegPath:      "ffmpeg",
		FFprobePath:     "ffprobe",
		Timeout:         time.Minute,
		Runner:          runner,
		Now:             fixedNow,
	})
	if err != nil {
		t.Fatalf("NewFFmpegProcessor() error = %v", err)
	}

	result, err := processor.Process(context.Background(), processingJob(), domain.ProcessingAttempt{AttemptNo: 2})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}

	if result.ProcessedObjectKey != "processed/vid_123/source.mp4" {
		t.Fatalf("ProcessedObjectKey = %q", result.ProcessedObjectKey)
	}
	if result.ThumbnailObjectKey != "thumbnails/vid_123/poster.jpg" {
		t.Fatalf("ThumbnailObjectKey = %q", result.ThumbnailObjectKey)
	}
	if result.DurationMs != 12340 || result.Width != 1280 || result.Height != 720 || result.SizeBytes != int64(len("processed-video")) {
		t.Fatalf("result metadata = %#v", result)
	}
	if len(store.uploads) != 2 {
		t.Fatalf("uploads = %d, want 2", len(store.uploads))
	}
	assertUpload(t, store.uploads[0], "processed-videos", "processed/vid_123/source.mp4", "video/mp4", "processed-video")
	assertUpload(t, store.uploads[1], "thumbnails", "thumbnails/vid_123/poster.jpg", "image/jpeg", "thumbnail")
	if len(runner.calls) != 3 {
		t.Fatalf("command count = %d, want 3", len(runner.calls))
	}
}

func TestFFmpegProcessorMapsCommandFailure(t *testing.T) {
	store := &fakeObjectStore{downloadBody: []byte("raw-video")}
	runner := &fakeRunner{
		onRun: func(name string, args []string) (CommandResult, error) {
			if name == "ffprobe" {
				return CommandResult{Stdout: []byte(`{"streams":[{"codec_type":"video","width":640,"height":360}],"format":{"duration":"1.000000"}}`)}, nil
			}
			return CommandResult{Stderr: []byte("invalid data found when processing input"), ExitCode: 1}, errors.New("exit status 1")
		},
	}
	processor, err := NewFFmpegProcessor(FFmpegConfig{
		ObjectStore:     store,
		ProcessedBucket: "processed-videos",
		ThumbnailBucket: "thumbnails",
		Timeout:         time.Minute,
		Runner:          runner,
	})
	if err != nil {
		t.Fatalf("NewFFmpegProcessor() error = %v", err)
	}

	_, err = processor.Process(context.Background(), processingJob(), domain.ProcessingAttempt{AttemptNo: 1})
	var processingErr domain.ProcessingError
	if !errors.As(err, &processingErr) {
		t.Fatalf("Process() error = %T, want ProcessingError", err)
	}
	if processingErr.Code != domain.CodeFFmpegFailed || processingErr.Retryable {
		t.Fatalf("processingErr = %#v", processingErr)
	}
	if !strings.Contains(processingErr.Message, "invalid data found") {
		t.Fatalf("message = %q", processingErr.Message)
	}
}

func TestSanitizeStderrTruncatesOutput(t *testing.T) {
	raw := []byte(strings.Repeat("x", maxStderrExcerptBytes+100))
	excerpt := sanitizeStderr(raw)
	if len(excerpt) <= maxStderrExcerptBytes {
		t.Fatalf("excerpt was not marked as truncated")
	}
	if !strings.HasSuffix(excerpt, "...truncated") {
		t.Fatalf("excerpt suffix = %q", excerpt[len(excerpt)-20:])
	}
}

func processingJob() domain.ProcessingJob {
	return domain.ProcessingJob{
		ID:             "job_123",
		VideoID:        "vid_123",
		OwnerID:        "usr_123",
		InputBucket:    "raw-videos",
		InputObjectKey: "raw/vid_123/source.mov",
		ContentType:    "video/quicktime",
		SizeBytes:      9,
		MaxAttempts:    3,
	}
}

type fakeRunner struct {
	calls []fakeCommandCall
	onRun func(name string, args []string) (CommandResult, error)
}

type fakeCommandCall struct {
	name string
	args []string
}

func (f *fakeRunner) Run(ctx context.Context, name string, args ...string) (CommandResult, error) {
	f.calls = append(f.calls, fakeCommandCall{name: name, args: append([]string(nil), args...)})
	if f.onRun == nil {
		return CommandResult{}, nil
	}
	return f.onRun(name, args)
}

type fakeObjectStore struct {
	downloadBody []byte
	uploads      []fakeUpload
}

type fakeUpload struct {
	bucket      string
	objectKey   string
	contentType string
	body        []byte
}

func (f *fakeObjectStore) VerifyObject(context.Context, storage.VerifyObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{SizeBytes: int64(len(f.downloadBody))}, nil
}

func (f *fakeObjectStore) DownloadObject(context.Context, storage.ObjectRef) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(f.downloadBody)), nil
}

func (f *fakeObjectStore) UploadObject(_ context.Context, input storage.UploadObjectInput) (storage.ObjectMetadata, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	f.uploads = append(f.uploads, fakeUpload{
		bucket:      input.Bucket,
		objectKey:   input.ObjectKey,
		contentType: input.ContentType,
		body:        body,
	})
	return storage.ObjectMetadata{SizeBytes: int64(len(body)), ContentType: input.ContentType}, nil
}

func assertUpload(t *testing.T, upload fakeUpload, bucket string, objectKey string, contentType string, body string) {
	t.Helper()
	if upload.bucket != bucket || upload.objectKey != objectKey || upload.contentType != contentType || string(upload.body) != body {
		t.Fatalf("upload = %#v", upload)
	}
}

func containsArg(args []string, value string) bool {
	for _, arg := range args {
		if arg == value {
			return true
		}
	}
	return false
}

func fixedNow() time.Time {
	return time.Date(2026, 7, 3, 10, 0, 0, 0, time.UTC)
}
