package service

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"mime"
	"path/filepath"
	"strings"
	"time"

	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/domain"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/event"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/repository"
	"github.com/truong-devops/aiops-multiagent-rag-k8s/services/video-service/internal/storage"
)

type VideoService struct {
	store            repository.Store
	environment      string
	rawVideoBucket   string
	uploadURLBase    string
	uploadRequestTTL time.Duration
	presignedTTL     time.Duration
	uploadSigner     storage.UploadSigner
	objectVerifier   storage.ObjectVerifier
	metrics          MetricsRecorder
	logger           *slog.Logger
	now              func() time.Time
}

type Options struct {
	Environment      string
	RawVideoBucket   string
	UploadURLBase    string
	UploadRequestTTL time.Duration
	PresignedTTL     time.Duration
	UploadSigner     storage.UploadSigner
	ObjectVerifier   storage.ObjectVerifier
	Metrics          MetricsRecorder
	Logger           *slog.Logger
	Now              func() time.Time
}

type MetricsRecorder interface {
	RecordUploadRequest(outcome string)
	RecordUploadConfirmation(outcome string)
	RecordPresign(outcome string)
	RecordObjectVerification(outcome string)
	RecordStatusTransition(from string, to string, outcome string)
	RecordDBOperation(operation string, outcome string, duration time.Duration)
}

type Actor struct {
	UserID   string
	Roles    []string
	Internal bool
}

type CreateUploadRequestInput struct {
	OwnerID        string
	Title          string
	Description    string
	Visibility     string
	ContentType    string
	SizeBytes      int64
	ChecksumSHA256 string
	IdempotencyKey string
	Actor          Actor
	RequestID      string
	CorrelationID  string
}

type UploadIntent struct {
	Video         domain.Video
	UploadRequest domain.UploadRequest
	UploadURL     string
}

type ConfirmUploadedInput struct {
	VideoID         string
	UploadRequestID string
	SizeBytes       int64
	ChecksumSHA256  string
	Actor           Actor
	RequestID       string
	CorrelationID   string
}

type UpdateStatusInput struct {
	VideoID       string
	Status        string
	Reason        string
	ErrorCode     string
	Actor         Actor
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
	presignedTTL := options.PresignedTTL
	if presignedTTL <= 0 {
		presignedTTL = 15 * time.Minute
	}
	logger := options.Logger
	if logger == nil {
		logger = slog.Default()
	}
	return &VideoService{
		store:            store,
		environment:      defaultEnvironment(options.Environment),
		rawVideoBucket:   options.RawVideoBucket,
		uploadURLBase:    strings.TrimRight(options.UploadURLBase, "/"),
		uploadRequestTTL: ttl,
		presignedTTL:     presignedTTL,
		uploadSigner:     options.UploadSigner,
		objectVerifier:   options.ObjectVerifier,
		metrics:          options.Metrics,
		logger:           logger,
		now:              now,
	}
}

func (s *VideoService) CreateUploadRequest(ctx context.Context, input CreateUploadRequestInput) (UploadIntent, error) {
	ownerID := strings.TrimSpace(input.OwnerID)
	if ownerID == "" {
		ownerID = strings.TrimSpace(input.Actor.UserID)
	}
	if ownerID == "" {
		s.recordUploadRequest("unauthorized")
		return UploadIntent{}, domain.Unauthorized("User context is required.")
	}
	if !input.Actor.canWriteOwner(ownerID) && input.Actor.UserID != "" {
		s.recordUploadRequest("forbidden")
		return UploadIntent{}, domain.Forbidden("User cannot create upload requests for another owner.")
	}
	title := strings.TrimSpace(input.Title)
	if title == "" {
		s.recordUploadRequest("validation_error")
		return UploadIntent{}, domain.ValidationError("title is required.")
	}
	contentType := strings.TrimSpace(input.ContentType)
	if !validVideoContentType(contentType) {
		s.recordUploadRequest("validation_error")
		return UploadIntent{}, domain.ValidationError("content_type must be a supported video MIME type.")
	}
	if input.SizeBytes < 0 {
		s.recordUploadRequest("validation_error")
		return UploadIntent{}, domain.ValidationError("size_bytes must be greater than or equal to zero.")
	}
	visibility := strings.TrimSpace(input.Visibility)
	if visibility == "" {
		visibility = domain.VisibilityPrivate
	}
	if !domain.ValidVisibility(visibility) {
		s.recordUploadRequest("validation_error")
		return UploadIntent{}, domain.ValidationError("visibility is invalid.")
	}
	idempotencyKey := strings.TrimSpace(input.IdempotencyKey)
	if idempotencyKey != "" {
		startedAt := time.Now()
		existingVideo, existingUpload, err := s.store.FindUploadIntentByIdempotencyKey(ctx, ownerID, idempotencyKey)
		s.recordDBOperation("find_upload_intent_by_idempotency_key", startedAt, err)
		if err == nil {
			uploadURL, err := s.uploadURL(ctx, existingUpload)
			if err != nil {
				s.recordUploadRequest("presign_error")
				return UploadIntent{}, err
			}
			s.recordUploadRequest("reused")
			s.logger.Info(
				"upload intent reused by idempotency key",
				"video_id", existingVideo.ID,
				"upload_request_id", existingUpload.ID,
				"owner_id", existingVideo.OwnerID,
				"request_id", input.RequestID,
				"correlation_id", input.CorrelationID,
			)
			return UploadIntent{Video: existingVideo, UploadRequest: existingUpload, UploadURL: uploadURL}, nil
		}
		if !isNotFound(err) {
			s.recordUploadRequest("db_error")
			return UploadIntent{}, err
		}
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
		IdempotencyKey: idempotencyKey,
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
	startedAt := time.Now()
	if err := s.store.CreateVideoWithUploadRequest(ctx, video, upload, history); err != nil {
		s.recordDBOperation("create_upload_intent", startedAt, err)
		s.recordUploadRequest("db_error")
		return UploadIntent{}, err
	}
	s.recordDBOperation("create_upload_intent", startedAt, nil)
	uploadURL, err := s.uploadURL(ctx, upload)
	if err != nil {
		s.recordUploadRequest("presign_error")
		return UploadIntent{}, err
	}
	s.recordUploadRequest("created")
	s.logger.Info(
		"upload intent created",
		"video_id", video.ID,
		"upload_request_id", upload.ID,
		"owner_id", video.OwnerID,
		"request_id", input.RequestID,
		"correlation_id", input.CorrelationID,
	)
	return UploadIntent{Video: video, UploadRequest: upload, UploadURL: uploadURL}, nil
}

func (s *VideoService) ConfirmUploaded(ctx context.Context, input ConfirmUploadedInput) (domain.Video, error) {
	uploadID := strings.TrimSpace(input.UploadRequestID)
	if uploadID == "" {
		return domain.Video{}, domain.ValidationError("upload_request_id is required.")
	}
	startedAt := time.Now()
	upload, err := s.store.FindUploadRequestByID(ctx, uploadID)
	s.recordDBOperation("find_upload_request", startedAt, err)
	if err != nil {
		s.recordUploadConfirmation("db_error")
		return domain.Video{}, err
	}
	if !input.Actor.canReadOwner(upload.OwnerID) {
		s.recordUploadConfirmation("forbidden")
		return domain.Video{}, actorError(input.Actor, "User cannot confirm this upload request.")
	}
	if upload.Status != domain.UploadStatusCreated {
		s.recordUploadConfirmation("conflict")
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Upload request is not in created state.")
	}
	if strings.TrimSpace(input.VideoID) != "" && strings.TrimSpace(input.VideoID) != upload.VideoID {
		s.recordUploadConfirmation("validation_error")
		return domain.Video{}, domain.ValidationError("upload_request_id does not belong to the requested video.")
	}
	now := s.now().UTC()
	if now.After(upload.ExpiresAt) {
		upload.Status = domain.UploadStatusExpired
		upload.UpdatedAt = now
		startedAt = time.Now()
		err := s.store.SaveUploadRequest(ctx, upload)
		s.recordDBOperation("expire_upload_request", startedAt, err)
		s.recordUploadConfirmation("expired")
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Upload request has expired.")
	}

	startedAt = time.Now()
	video, err := s.store.FindVideoByID(ctx, upload.VideoID)
	s.recordDBOperation("find_video", startedAt, err)
	if err != nil {
		s.recordUploadConfirmation("db_error")
		return domain.Video{}, err
	}
	if !input.Actor.canReadOwner(video.OwnerID) {
		s.recordUploadConfirmation("forbidden")
		return domain.Video{}, actorError(input.Actor, "User cannot confirm this video upload.")
	}
	if !domain.CanTransitionVideo(video.Status, domain.VideoStatusUploaded) {
		s.recordUploadConfirmation("conflict")
		return domain.Video{}, domain.Conflict(domain.CodeInvalidVideoState, "Video cannot transition to uploaded.")
	}

	verifiedMetadata, err := s.verifyUploadedObject(ctx, upload)
	if err != nil {
		s.recordUploadConfirmation("object_verification_error")
		return domain.Video{}, err
	}
	if input.SizeBytes > 0 {
		if verifiedMetadata.SizeBytes > 0 && input.SizeBytes != verifiedMetadata.SizeBytes {
			s.recordObjectVerification("metadata_mismatch")
			s.recordUploadConfirmation("metadata_mismatch")
			return domain.Video{}, domain.Conflict(domain.CodeUploadObjectMismatch, "Uploaded object size does not match the confirmation request.")
		}
		video.SizeBytes = input.SizeBytes
		upload.SizeBytes = input.SizeBytes
	} else if verifiedMetadata.SizeBytes > 0 {
		video.SizeBytes = verifiedMetadata.SizeBytes
		upload.SizeBytes = verifiedMetadata.SizeBytes
	}
	if verifiedMetadata.ContentType != "" && !sameContentType(upload.ContentType, verifiedMetadata.ContentType) {
		s.recordObjectVerification("metadata_mismatch")
		s.recordUploadConfirmation("metadata_mismatch")
		return domain.Video{}, domain.Conflict(domain.CodeUploadObjectMismatch, "Uploaded object content type does not match the upload request.")
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
	outbox, err := event.NewVideoUploadedOutbox(video, s.environment, now)
	if err != nil {
		return domain.Video{}, err
	}
	startedAt = time.Now()
	if err := s.store.CompleteUpload(ctx, upload, video, history, outbox); err != nil {
		s.recordDBOperation("complete_upload", startedAt, err)
		s.recordUploadConfirmation("db_error")
		return domain.Video{}, err
	}
	s.recordDBOperation("complete_upload", startedAt, nil)
	s.recordUploadConfirmation("confirmed")
	s.recordStatusTransition(previousStatus, video.Status, "updated")
	s.logger.Info(
		"upload confirmed",
		"video_id", video.ID,
		"upload_request_id", upload.ID,
		"owner_id", video.OwnerID,
		"request_id", input.RequestID,
		"correlation_id", input.CorrelationID,
	)
	return video, nil
}

func (s *VideoService) GetVideo(ctx context.Context, id string) (domain.Video, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return domain.Video{}, domain.ValidationError("video_id is required.")
	}
	startedAt := time.Now()
	video, err := s.store.FindVideoByID(ctx, id)
	s.recordDBOperation("find_video", startedAt, err)
	return video, err
}

func (s *VideoService) GetVideoForActor(ctx context.Context, id string, actor Actor) (domain.Video, error) {
	video, err := s.GetVideo(ctx, id)
	if err != nil {
		return domain.Video{}, err
	}
	if !actor.canReadOwner(video.OwnerID) {
		return domain.Video{}, actorError(actor, "User cannot access this video.")
	}
	return video, nil
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
	startedAt := time.Now()
	videos, err := s.store.ListVideos(ctx, filter)
	s.recordDBOperation("list_videos", startedAt, err)
	return videos, err
}

func (s *VideoService) ListVideosForActor(ctx context.Context, filter repository.ListVideosFilter, actor Actor) ([]domain.Video, error) {
	if !actor.isInternalOrAdmin() {
		if strings.TrimSpace(actor.UserID) == "" {
			return nil, domain.Unauthorized("User context is required.")
		}
		filter.OwnerID = actor.UserID
	}
	return s.ListVideos(ctx, filter)
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
	if !input.Actor.canUpdateVideoStatus(video.OwnerID, status) {
		s.recordStatusTransition(video.Status, status, "forbidden")
		return domain.Video{}, actorError(input.Actor, "User cannot update this video status.")
	}
	if !domain.CanTransitionVideo(video.Status, status) {
		s.recordStatusTransition(video.Status, status, "conflict")
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
	startedAt := time.Now()
	if err := s.store.SaveVideoStatus(ctx, video, history); err != nil {
		s.recordDBOperation("save_video_status", startedAt, err)
		s.recordStatusTransition(previousStatus, status, "db_error")
		return domain.Video{}, err
	}
	s.recordDBOperation("save_video_status", startedAt, nil)
	s.recordStatusTransition(previousStatus, status, "updated")
	s.logger.Info(
		"video status updated",
		"video_id", video.ID,
		"owner_id", video.OwnerID,
		"status", video.Status,
		"request_id", input.RequestID,
		"correlation_id", input.CorrelationID,
	)
	return video, nil
}

func (s *VideoService) verifyUploadedObject(ctx context.Context, upload domain.UploadRequest) (storage.ObjectMetadata, error) {
	if s.objectVerifier == nil {
		return storage.ObjectMetadata{}, nil
	}
	metadata, err := s.objectVerifier.VerifyObject(ctx, storage.VerifyObjectInput{
		Bucket:    upload.Bucket,
		ObjectKey: upload.ObjectKey,
		Now:       s.now().UTC(),
	})
	if err != nil {
		s.recordObjectVerification("error")
		s.logger.Error(
			"upload object verification failed",
			"video_id", upload.VideoID,
			"upload_request_id", upload.ID,
			"owner_id", upload.OwnerID,
			"error", err,
		)
		return storage.ObjectMetadata{}, err
	}
	s.recordObjectVerification("verified")
	s.logger.Info(
		"upload object verified",
		"video_id", upload.VideoID,
		"upload_request_id", upload.ID,
		"owner_id", upload.OwnerID,
		"size_bytes", metadata.SizeBytes,
		"content_type", metadata.ContentType,
	)
	return metadata, nil
}

func (s *VideoService) Ready(ctx context.Context) error {
	return s.store.Ping(ctx)
}

func (s *VideoService) uploadURL(ctx context.Context, upload domain.UploadRequest) (string, error) {
	if s.uploadSigner != nil {
		url, err := s.uploadSigner.PresignPutObject(ctx, storage.PresignPutObjectInput{
			Bucket:      upload.Bucket,
			ObjectKey:   upload.ObjectKey,
			ContentType: upload.ContentType,
			Expires:     s.presignedTTL,
			Now:         s.now().UTC(),
		})
		if err != nil {
			s.recordPresign("error")
			return "", err
		}
		s.recordPresign("success")
		return url, nil
	}
	if s.uploadURLBase == "" {
		return "", nil
	}
	s.recordPresign("local_fallback")
	return fmt.Sprintf("%s/%s/%s", s.uploadURLBase, upload.Bucket, upload.ObjectKey), nil
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

func sameContentType(left string, right string) bool {
	left = strings.ToLower(strings.TrimSpace(strings.Split(left, ";")[0]))
	right = strings.ToLower(strings.TrimSpace(strings.Split(right, ";")[0]))
	return left == right
}

func defaultEnvironment(environment string) string {
	environment = strings.TrimSpace(environment)
	if environment == "" {
		return "local"
	}
	return environment
}

func (a Actor) isInternalOrAdmin() bool {
	if a.Internal {
		return true
	}
	for _, role := range a.Roles {
		role = strings.ToLower(strings.TrimSpace(role))
		if role == "admin" || role == "video_admin" {
			return true
		}
	}
	return false
}

func (a Actor) canReadOwner(ownerID string) bool {
	if a.isInternalOrAdmin() {
		return true
	}
	return strings.TrimSpace(a.UserID) != "" && strings.TrimSpace(a.UserID) == strings.TrimSpace(ownerID)
}

func (a Actor) canWriteOwner(ownerID string) bool {
	if a.isInternalOrAdmin() {
		return true
	}
	return strings.TrimSpace(a.UserID) == "" || strings.TrimSpace(a.UserID) == strings.TrimSpace(ownerID)
}

func (a Actor) canUpdateVideoStatus(ownerID string, status string) bool {
	if a.isInternalOrAdmin() {
		return true
	}
	return strings.TrimSpace(a.UserID) != "" &&
		strings.TrimSpace(a.UserID) == strings.TrimSpace(ownerID) &&
		strings.TrimSpace(status) == domain.VideoStatusDeleted
}

func actorError(actor Actor, forbiddenMessage string) error {
	if strings.TrimSpace(actor.UserID) == "" && !actor.Internal {
		return domain.Unauthorized("User context is required.")
	}
	return domain.Forbidden(forbiddenMessage)
}

func isNotFound(err error) bool {
	var appErr *domain.AppError
	return errors.As(err, &appErr) && appErr.Status == 404
}

func (s *VideoService) recordUploadRequest(outcome string) {
	if s.metrics != nil {
		s.metrics.RecordUploadRequest(outcome)
	}
}

func (s *VideoService) recordUploadConfirmation(outcome string) {
	if s.metrics != nil {
		s.metrics.RecordUploadConfirmation(outcome)
	}
}

func (s *VideoService) recordPresign(outcome string) {
	if s.metrics != nil {
		s.metrics.RecordPresign(outcome)
	}
}

func (s *VideoService) recordObjectVerification(outcome string) {
	if s.metrics != nil {
		s.metrics.RecordObjectVerification(outcome)
	}
}

func (s *VideoService) recordStatusTransition(from string, to string, outcome string) {
	if s.metrics != nil {
		s.metrics.RecordStatusTransition(from, to, outcome)
	}
}

func (s *VideoService) recordDBOperation(operation string, startedAt time.Time, err error) {
	if s.metrics == nil {
		return
	}
	outcome := "success"
	if err != nil && !isNotFound(err) {
		outcome = "error"
	}
	if isNotFound(err) {
		outcome = "not_found"
	}
	s.metrics.RecordDBOperation(operation, outcome, time.Since(startedAt))
}
