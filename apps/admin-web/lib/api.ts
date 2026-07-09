import type {
  AuthResult,
  Envelope,
  FeedItem,
  HealthCheck,
  HealthStatus,
  Incident,
  LiveSession,
  UploadIntent,
  User,
  VideoItem
} from "@/lib/types";

const apiBaseURL = (process.env.NEXT_PUBLIC_API_BASE_URL || "http://localhost:8080").replace(/\/$/, "");

export class ApiError extends Error {
  status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "ApiError";
    this.status = status;
  }
}

type ListOptions = {
  limit?: number;
  cursor?: string;
  status?: string;
  owner_id?: string;
};

type RequestOptions = RequestInit & {
  token?: string;
};

async function apiRequest<T>(path: string, init: RequestOptions = {}): Promise<T> {
  const { token, ...requestInit } = init;
  const headers = new Headers(init.headers);
  headers.set("Accept", "application/json");
  if (init.body && !headers.has("Content-Type")) {
    headers.set("Content-Type", "application/json");
  }
  if (token) {
    headers.set("Authorization", `Bearer ${token}`);
  }

  const response = await fetch(`${apiBaseURL}${path}`, {
    ...requestInit,
    headers,
    cache: "no-store"
  });

  if (!response.ok) {
    let message = response.statusText || "Request failed";
    try {
      const payload = (await response.json()) as { error?: { message?: string; code?: string } };
      message = payload.error?.message || payload.error?.code || message;
    } catch {
      // Keep the HTTP status text when the upstream returns plain text.
    }
    throw new ApiError(message, response.status);
  }

  if (response.status === 204) {
    return undefined as T;
  }

  const contentType = response.headers.get("content-type") || "";
  if (!contentType.includes("application/json")) {
    return (await response.text()) as T;
  }
  return (await response.json()) as T;
}

export async function getHealth(names: string[]): Promise<HealthCheck[]> {
  const checks = await Promise.all(
    names.map(async (name): Promise<HealthCheck> => {
      if (name !== "api-gateway") {
        return {
          name,
          status: "unknown",
          message: "Upstream readiness is not exposed through the gateway yet."
        };
      }
      try {
        await apiRequest<unknown>("/readyz");
        return { name, status: "ready" };
      } catch (error) {
        const status: HealthStatus = error instanceof ApiError && error.status < 500 ? "degraded" : "down";
        return {
          name,
          status,
          message: error instanceof Error ? error.message : "Health check failed"
        };
      }
    })
  );
  return checks;
}

export async function login(email: string, password: string): Promise<AuthResult> {
  const envelope = await apiRequest<Envelope<AuthResult>>("/api/v1/auth/login", {
    method: "POST",
    body: JSON.stringify({ email, password })
  });
  return envelope.data;
}

export async function register(input: {
  email: string;
  username: string;
  display_name: string;
  password: string;
}): Promise<User> {
  const envelope = await apiRequest<Envelope<{ user: User }>>("/api/v1/auth/register", {
    method: "POST",
    body: JSON.stringify(input)
  });
  return envelope.data.user;
}

export async function currentUser(token: string): Promise<User> {
  const envelope = await apiRequest<Envelope<{ user: User }>>("/api/v1/users/me", { token });
  return envelope.data.user;
}

export async function listVideos(options: ListOptions = {}, token?: string): Promise<VideoItem[]> {
  const envelope = await apiRequest<Envelope<VideoItem[]>>(`/api/v1/videos${toQuery(options)}`, { token });
  return envelope.data || [];
}

export async function createUploadRequest(
  input: {
    title: string;
    description: string;
    visibility: string;
    content_type: string;
    size_bytes: number;
    checksum_sha256: string;
  },
  token: string
): Promise<UploadIntent> {
  const envelope = await apiRequest<Envelope<UploadIntent>>("/api/v1/videos/upload-requests", {
    method: "POST",
    token,
    headers: { "Idempotency-Key": crypto.randomUUID() },
    body: JSON.stringify(input)
  });
  return envelope.data;
}

export async function confirmUploaded(
  videoID: string,
  input: { upload_request_id: string; size_bytes: number; checksum_sha256: string },
  token: string
): Promise<VideoItem> {
  const envelope = await apiRequest<Envelope<{ video: VideoItem }>>(`/api/v1/videos/${videoID}/uploaded`, {
    method: "POST",
    token,
    body: JSON.stringify(input)
  });
  return envelope.data.video;
}

export async function uploadObject(uploadURL: string, file: File): Promise<void> {
  const response = await fetch(uploadURL, {
    method: "PUT",
    headers: { "Content-Type": file.type || "application/octet-stream" },
    body: file
  });
  if (!response.ok) {
    throw new ApiError("Object upload failed. Check MinIO CORS and presigned URL expiry.", response.status);
  }
}

export async function listFeed(options: ListOptions = {}): Promise<FeedItem[]> {
  const envelope = await apiRequest<Envelope<FeedItem[]>>(`/api/v1/feed${toQuery(options)}`);
  return envelope.data || [];
}

export async function listLiveSessions(options: ListOptions = {}, token?: string): Promise<LiveSession[]> {
  const envelope = await apiRequest<Envelope<LiveSession[]>>(`/api/v1/live-sessions${toQuery(options)}`, { token });
  return envelope.data || [];
}

export async function createLiveSession(
  input: { title: string; description: string; scheduled_at?: string },
  token: string
): Promise<LiveSession> {
  const envelope = await apiRequest<Envelope<LiveSession>>("/api/v1/live-sessions", {
    method: "POST",
    token,
    body: JSON.stringify(input)
  });
  return envelope.data;
}

export async function transitionLiveSession(id: string, action: "start" | "end", token: string): Promise<LiveSession> {
  const envelope = await apiRequest<Envelope<LiveSession>>(`/api/v1/live-sessions/${id}/${action}`, {
    method: "POST",
    token
  });
  return envelope.data;
}

export async function createIncident(
  input: {
    service: string;
    namespace: string;
    symptom: string;
    severity: string;
    started_at: string;
    time_window: string;
  },
  token: string
): Promise<Incident> {
  const envelope = await apiRequest<Envelope<Incident>>("/api/v1/incidents", {
    method: "POST",
    token,
    body: JSON.stringify(input)
  });
  return envelope.data;
}

export async function sha256Hex(file: File): Promise<string> {
  const bytes = await file.arrayBuffer();
  const digest = await crypto.subtle.digest("SHA-256", bytes);
  return Array.from(new Uint8Array(digest))
    .map((value) => value.toString(16).padStart(2, "0"))
    .join("");
}

function toQuery(options: ListOptions) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(options)) {
    if (value !== undefined && value !== "") {
      params.set(key, String(value));
    }
  }
  const value = params.toString();
  return value ? `?${value}` : "";
}
