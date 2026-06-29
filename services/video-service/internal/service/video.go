package service

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
)

type VideoService struct {
	store            repository.Store
	rawVideoBucket   string
	uploadURLBase    string
	uploadRequestTTL time.Duration
	now              func() time.Time
}

type Options struct {
	RawVideoBucket   string
	UploadURLBase    string
	UploadRequestTTL time.Duration
	Now              func() time.Time
}

type CreateUploadRequestInput struct {
	OwnerID        string
	Title          string
	Description    string
	Visibility     string
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	RequestID      string
	CorrelationID  string
}

type UploadIntent struct {
	Video         domain.Video
	UploadRequest domain.UploadRequest
	UploadURL     string
}

type ConfirmUploadedInput struct {
	UploadRequestID string
	SizeBytes       int64
	ChecksumSHA256  string
	RequestID       string
	CorrelationID   string
}

type UpdateStatusInput struct {
	VideoID       string
	Status        string
	Reason        string
	ErrorCode     string
	RequestID     string
	CorrelationID string
}

func NewVideoService(store repository.Store, options Options) *VideoService {
	now := options.Now
	if now == nil {
		now = time.Now
	}
	ttl := options.UploadRequestTTL
	if ttl <= 0 {
		ttl = 30 * time.Minute
	}
	return &VideoService{
		store:            store,
		rawVideoBucket:   options.RawVideoBucket,
		uploadURLBase:    strings.TrimRight(options.UploadURLBase, "/"),
		uploadRequestTTL: ttl,
		now:              now,
	}
}

func (s *VideoService) CreateUploadRequest(ctx context.Context, input CreateUploadRequestInput) (UploadIntent, error) {
	ownerID := strings.TrimSpace(input.OwnerID)
	if ownerID == "" {
		return UploadIntent{}, domain.Unauthorized("User context is required.")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		return UploadIntent{}, domain.ValidationError("title is required.")
	}
	contentType := strings.TrimSpace(input.ContentType)
	if !validVideoContentType(contentType) {
		return UploadIntent{}, domain.ValidationError("content_type must be a supported video MIME type.")
	}
	if input.SizeBytes < 0 {
		return UploadIntent{}, domain.ValidationError("size_bytes must be greater than or equal to zero.")
	}
	visibility := strings.TrimSpace(input.Visibility)
	if visibility == "" {
		visibility = domain.VisibilityPrivate
	}
	if !domain.ValidVisibility(visibility) {
		return UploadIntent{}, domain.ValidationError("visibility is invalid.")
	}

	now := s.now().UTC()
	videoID := domain.NewID("vid")
	uploadID := domain.NewID("upl")
	objectKey := rawObjectKey(videoID, contentType)

	video := domain.Video{
		ID:                videoID,
		OwnerID:           ownerID,
		Title:             title,
		Description:       strings.TrimSpace(input.Description),
		Status:            domain.VideoStatusDraft,
		Visibility:        visibility,
		RawObjectKey:      objectKey,
		ContentType:       contentType,
		SizeBytes:         input.SizeBytes,
		CreatedAt:         now,
		UpdatedAt:         now,
		LastRequestID:     input.RequestID,
		LastCorrelationID: input.CorrelationID,
	}
	upload := domain.UploadRequest{
		ID:             uploadID,
		VideoID:        videoID,
		OwnerID:        ownerID,
		Bucket:         s.rawVideoBucket,
		ObjectKey:      objectKey,
		Status:         domain.UploadStatusCreated,
		ContentType:    contentType,
		SizeBytes:      input.SizeBytes,
		ChecksumSHA256: strings.TrimSpace(input.ChecksumSHA256),
		ExpiresAt:      now.Add(s.uploadRequestTTL),
		CreatedAt:      now,
		UpdatedAt:      now,
		RequestID:      input.RequestID,
		CorrelationID:  input.CorrelationID,
	}
	history := domain.StatusHistory{
		ID:            domain.NewID("vsh"),
		VideoID:       videoID,
		NewStatus:     domain.VideoStatusDraft,
		Reason:        "upload_request_created",
		RequestID:     input.RequestID,
		CorrelationID: input.CorrelationID,
		CreatedAt:     now,
	}
	if err := s.store.CreateVideoWithUploadRequest(ctx, video, upload, history); err != nil {
		return UploadIntent{}, err
	}
	return UploadIntent{Video: video, UploadRequest: upload, UploadURL: s.uploadURL(upload)}, nil
}

func (s *VideoService) ConfirmUploaded(ctx context.Context, input ConfirmUploadedInput) (domain.Video, error) {
	uploadID := strings.TrimSpace(input.UploadRequestID)
	if uploadID == "" {
		return domain.Video{}, domain.ValidationError("upload_request_id is required.")
	}
	upload, err := s.store.FindUploadRequestByID(ctx, uploadID)
	if err != nil {
		return domain.Video{}, err
	}
	if upload.Status != domain.UploadStatusCreated {
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Upload request is not in created state.")
	}
	now := s.now().UTC()
	if now.After(upload.ExpiresAt) {
		upload.Status = domain.UploadStatusExpired
		upload.UpdatedAt = now
		_ = s.store.SaveUploadRequest(ctx, upload)
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Upload request has expired.")
	}

	video, err := s.store.FindVideoByID(ctx, upload.VideoID)
	if err != nil {
		return domain.Video{}, err
	}
	if !domain.CanTransitionVideo(video.Status, domain.VideoStatusUploaded) {
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Video cannot transition to uploaded.")
	}

	if input.SizeBytes > 0 {
		video.SizeBytes = input.SizeBytes
		upload.SizeBytes = input.SizeBytes
	}
	if strings.TrimSpace(input.ChecksumSHA256) != "" {
		upload.ChecksumSHA256 = strings.TrimSpace(input.ChecksumSHA256)
	}
	completedAt := now
	upload.Status = domain.UploadStatusUploaded
	upload.CompletedAt = &completedAt
	upload.UpdatedAt = now

	previousStatus := video.Status
	video.Status = domain.VideoStatusUploaded
	video.UpdatedAt = now
	video.LastRequestID = input.RequestID
	video.LastCorrelationID = input.CorrelationID
	history := domain.StatusHistory{
		ID:             domain.NewID("vsh"),
		VideoID:        video.ID,
		PreviousStatus: previousStatus,
		NewStatus:      video.Status,
		Reason:         "upload_confirmed",
		RequestID:      input.RequestID,
		CorrelationID:  input.CorrelationID,
		CreatedAt:      now,
	}
	if err := s.store.CompleteUpload(ctx, upload, video, history); err != nil {
		return domain.Video{}, err
	}
	return video, nil
}

func (s *VideoService) GetVideo(ctx context.Context, id string) (domain.Video, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Video{}, domain.ValidationError("video_id is required.")
	}
	return s.store.FindVideoByID(ctx, id)
}

func (s *VideoService) ListVideos(ctx context.Context, filter repository.ListVideosFilter) ([]domain.Video, error) {
	if filter.Status != "" && !domain.ValidVideoStatus(filter.Status) {
		return nil, domain.ValidationError("status filter is invalid.")
	}
	if filter.Limit <= 0 {
		filter.Limit = 20
	}
	if filter.Limit > 100 {
		filter.Limit = 100
	}
	return s.store.ListVideos(ctx, filter)
}

func (s *VideoService) UpdateStatus(ctx context.Context, input UpdateStatusInput) (domain.Video, error) {
	video, err := s.GetVideo(ctx, input.VideoID)
	if err != nil {
		return domain.Video{}, err
	}
	status := strings.TrimSpace(input.Status)
	if !domain.ValidVideoStatus(status) {
		return domain.Video{}, domain.ValidationError("status is invalid.")
	}
	if !domain.CanTransitionVideo(video.Status, status) {
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Video state transition is invalid.")
	}
	now := s.now().UTC()
	previousStatus := video.Status
	video.Status = status
	video.UpdatedAt = now
	video.LastRequestID = input.RequestID
	video.LastCorrelationID = input.CorrelationID
	if status == domain.VideoStatusFailed {
		video.ProcessingErrorCode = strings.TrimSpace(input.ErrorCode)
	}
	if status == domain.VideoStatusReady && video.Visibility == domain.VisibilityPublic {
		publishedAt := now
		video.PublishedAt = &publishedAt
	}
	if status == domain.VideoStatusDeleted {
		deletedAt := now
		video.DeletedAt = &deletedAt
	}
	history := domain.StatusHistory{
		ID:             domain.NewID("vsh"),
		VideoID:        video.ID,
		PreviousStatus: previousStatus,
		NewStatus:      status,
		Reason:         strings.TrimSpace(input.Reason),
		ErrorCode:      strings.TrimSpace(input.ErrorCode),
		RequestID:      input.RequestID,
		CorrelationID:  input.CorrelationID,
		CreatedAt:      now,
	}
	if err := s.store.SaveVideoStatus(ctx, video, history); err != nil {
		return domain.Video{}, err
	}
	return video, nil
}

func (s *VideoService) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *VideoService) uploadURL(upload domain.UploadRequest) string {
	if s.uploadURLBase == "" {
		return ""
	}
	return fmt.Sprintf("%s/%s/%s", s.uploadURLBase, upload.Bucket, upload.ObjectKey)
}

func rawObjectKey(videoID string, contentType string) string {
	extensions, _ := mime.ExtensionsByType(contentType)
	extension := ".bin"
	if len(extensions) > 0 {
		extension = extensions[0]
	}
	extension = strings.ToLower(filepath.Ext("file" + extension))
	if extension == "" {
		extension = ".bin"
	}
	return fmt.Sprintf("raw/%s/source%s", videoID, extension)
}

func validVideoContentType(contentType string) bool {
	contentType = strings.ToLower(strings.TrimSpace(contentType))
	return strings.HasPrefix(contentType, "video/")
}
