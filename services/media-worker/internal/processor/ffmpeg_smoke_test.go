//go:build smoke

package processor

import (
	"bytes"
	"context"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/media-worker/internal/storage"
)

func TestFFmpegProcessorSmoke(t *testing.T) {
	ffmpegPath := envOrDefault("FFMPEG_PATH", "ffmpeg")
	ffprobePath := envOrDefault("FFPROBE_PATH", "ffprobe")
	if _, err := exec.LookPath(ffmpegPath); err != nil {
		t.Skipf("ffmpeg binary not available: %v", err)
	}
	if _, err := exec.LookPath(ffprobePath); err != nil {
		t.Skipf("ffprobe binary not available: %v", err)
	}

	dir := t.TempDir()
	rawPath := filepath.Join(dir, "sample.mp4")
	cmd := exec.Command(ffmpegPath,
		"-y",
		"-f", "lavfi",
		"-i", "testsrc=size=128x72:rate=15:duration=1",
		"-pix_fmt", "yuv420p",
		rawPath,
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("create sample video: %v\n%s", err, string(output))
	}
	raw, err := os.ReadFile(rawPath)
	if err != nil {
		t.Fatalf("read sample video: %v", err)
	}
	store := &smokeObjectStore{downloadBody: raw}
	processor, err := NewFFmpegProcessor(FFmpegConfig{
		ObjectStore:     store,
		ProcessedBucket: "processed-videos",
		ThumbnailBucket: "thumbnails",
		FFmpegPath:      ffmpegPath,
		FFprobePath:     ffprobePath,
		Timeout:         30 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewFFmpegProcessor() error = %v", err)
	}

	result, err := processor.Process(context.Background(), domain.ProcessingJob{
		ID:             "job_smoke",
		VideoID:        "vid_smoke",
		OwnerID:        "usr_smoke",
		InputBucket:    "raw-videos",
		InputObjectKey: "raw/vid_smoke/source.mp4",
		ContentType:    "video/mp4",
		SizeBytes:      int64(len(raw)),
	}, domain.ProcessingAttempt{ID: "att_smoke", AttemptNo: 1})
	if err != nil {
		t.Fatalf("Process() error = %v", err)
	}
	if result.ProcessedObjectKey == "" || result.ThumbnailObjectKey == "" || result.SizeBytes <= 0 {
		t.Fatalf("result = %#v", result)
	}
	if len(store.uploads) != 2 {
		t.Fatalf("uploads = %d, want 2", len(store.uploads))
	}
	for _, upload := range store.uploads {
		if len(upload.body) == 0 {
			t.Fatalf("empty upload: %#v", upload)
		}
	}
}

type smokeObjectStore struct {
	downloadBody []byte
	uploads      []smokeUpload
}

type smokeUpload struct {
	bucket    string
	objectKey string
	body      []byte
}

func (s *smokeObjectStore) VerifyObject(context.Context, storage.VerifyObjectInput) (storage.ObjectMetadata, error) {
	return storage.ObjectMetadata{SizeBytes: int64(len(s.downloadBody)), ContentType: "video/mp4"}, nil
}

func (s *smokeObjectStore) DownloadObject(context.Context, storage.ObjectRef) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(s.downloadBody)), nil
}

func (s *smokeObjectStore) UploadObject(_ context.Context, input storage.UploadObjectInput) (storage.ObjectMetadata, error) {
	body, err := io.ReadAll(input.Body)
	if err != nil {
		return storage.ObjectMetadata{}, err
	}
	s.uploads = append(s.uploads, smokeUpload{bucket: input.Bucket, objectKey: input.ObjectKey, body: body})
	return storage.ObjectMetadata{SizeBytes: int64(len(body)), ContentType: input.ContentType}, nil
}

func envOrDefault(key string, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
