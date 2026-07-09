export type Page = {
  limit: number;
  next_cursor?: string;
  has_more: boolean;
};

export type Envelope<T> = {
  data: T;
  page?: Page;
  request_id?: string;
};

export type HealthStatus = "ready" | "degraded" | "down" | "unknown";

export type HealthCheck = {
  name: string;
  status: HealthStatus;
  message?: string;
};

export type User = {
  id: string;
  email: string;
  username?: string;
  display_name?: string;
  status?: string;
  roles?: string[];
  created_at?: string;
};

export type AuthResult = {
  access_token: string;
  refresh_token: string;
  token_type: string;
  expires_in: number;
  user: User;
};

export type VideoItem = {
  id: string;
  owner_id: string;
  title: string;
  description: string;
  status: string;
  visibility: string;
  raw_object_key?: string;
  processed_object_key?: string;
  thumbnail_object_key?: string;
  content_type?: string;
  size_bytes?: number;
  duration_ms?: number;
  width?: number;
  height?: number;
  processing_error_code?: string;
  published_at?: string | null;
  deleted_at?: string | null;
  created_at: string;
  updated_at: string;
};

export type UploadRequest = {
  id: string;
  video_id: string;
  bucket: string;
  object_key: string;
  status: string;
  content_type: string;
  size_bytes: number;
  checksum_sha256: string;
  expires_at: string;
  upload_url: string;
};

export type UploadIntent = {
  video: VideoItem;
  upload_request: UploadRequest;
};

export type FeedItem = {
  video_id: string;
  owner: {
    id: string;
    display_name: string;
  };
  title: string;
  description: string;
  thumbnail_object_key: string;
  playback_object_key: string;
  duration_ms: number;
  like_count: number;
  comment_count: number;
  ready_at: string;
};

export type LiveSession = {
  id: string;
  owner_id: string;
  creator_id: string;
  title: string;
  description: string;
  status: string;
  stream_key?: string;
  ingest_url: string;
  playback_url: string;
  scheduled_at?: string;
  started_at?: string;
  ended_at?: string;
  failure_code?: string;
  created_at: string;
  updated_at: string;
};

export type Incident = {
  id: string;
  status: string;
  service: string;
  namespace?: string;
  symptom: string;
  severity?: string;
  started_at?: string;
};
