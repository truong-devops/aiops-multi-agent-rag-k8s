"use client";

import {
  Activity,
  AlertCircle,
  CheckCircle2,
  Clock3,
  FileVideo,
  Gauge,
  KeyRound,
  ListFilter,
  LogOut,
  Radio,
  RefreshCw,
  Search,
  ShieldCheck,
  Siren,
  UploadCloud,
  UserPlus,
  Video,
  X
} from "lucide-react";
import type { FormEvent, ReactNode } from "react";
import { useCallback, useEffect, useMemo, useState } from "react";
import {
  ApiError,
  confirmUploaded,
  createIncident,
  createLiveSession,
  createUploadRequest,
  currentUser,
  getHealth,
  listFeed,
  listLiveSessions,
  listVideos,
  login,
  register,
  sha256Hex,
  transitionLiveSession,
  uploadObject
} from "@/lib/api";
import type { AuthResult, FeedItem, HealthCheck, LiveSession, User, VideoItem } from "@/lib/types";

type Tab = "overview" | "videos" | "live" | "aiops" | "account";

type LoadState = {
  health: HealthCheck[];
  videos: VideoItem[];
  feed: FeedItem[];
  liveSessions: LiveSession[];
  loading: boolean;
  error: string;
  updatedAt: string;
};

const initialState: LoadState = {
  health: [],
  videos: [],
  feed: [],
  liveSessions: [],
  loading: true,
  error: "",
  updatedAt: ""
};

const serviceNames = ["api-gateway", "identity", "video", "feed-social", "live", "aiops"];

export default function DashboardPage() {
  const [activeTab, setActiveTab] = useState<Tab>("overview");
  const [state, setState] = useState<LoadState>(initialState);
  const [query, setQuery] = useState("");
  const [ownerFilter, setOwnerFilter] = useState("");
  const [videoStatus, setVideoStatus] = useState("");
  const [token, setToken] = useState("");
  const [refreshToken, setRefreshToken] = useState("");
  const [user, setUser] = useState<User | null>(null);
  const [message, setMessage] = useState("");
  const [busyAction, setBusyAction] = useState("");

  useEffect(() => {
    const storedToken = localStorage.getItem("admin_access_token") || "";
    const storedRefreshToken = localStorage.getItem("admin_refresh_token") || "";
    setToken(storedToken);
    setRefreshToken(storedRefreshToken);
    if (storedToken) {
      currentUser(storedToken)
        .then(setUser)
        .catch(() => {
          localStorage.removeItem("admin_access_token");
          localStorage.removeItem("admin_refresh_token");
        });
    }
  }, []);

  const refresh = useCallback(async (nextToken = token) => {
    await Promise.resolve();
    setState((current) => ({ ...current, loading: true, error: "" }));
    try {
      const [health, videos, feed, liveSessions] = await Promise.all([
        getHealth(serviceNames),
        nextToken ? listVideos({ limit: 50 }, nextToken) : Promise.resolve([]),
        listFeed({ limit: 12 }),
        nextToken ? listLiveSessions({ limit: 20 }, nextToken) : Promise.resolve([])
      ]);
      setState({
        health,
        videos,
        feed,
        liveSessions,
        loading: false,
        error: "",
        updatedAt: new Date().toLocaleTimeString()
      });
    } catch (error) {
      setState((current) => ({ ...current, loading: false, error: errorMessage(error) }));
    }
  }, [token]);

  useEffect(() => {
    void refresh();
  }, [refresh]);

  useEffect(() => {
    if (token) {
      void refresh(token);
    }
  }, [refresh, token]);

  const filteredVideos = useMemo(() => {
    const keyword = query.trim().toLowerCase();
    return state.videos.filter((video) => {
      const matchesSearch =
        !keyword ||
        [video.id, video.owner_id, video.title, video.status, video.visibility]
          .filter(Boolean)
          .some((value) => value.toLowerCase().includes(keyword));
      const matchesStatus = !videoStatus || video.status === videoStatus;
      const ownerKeyword = ownerFilter.trim().toLowerCase();
      const matchesOwner = !ownerKeyword || video.owner_id.toLowerCase().includes(ownerKeyword);
      return matchesSearch && matchesStatus && matchesOwner;
    });
  }, [ownerFilter, query, state.videos, videoStatus]);

  const readyVideos = state.videos.filter((video) => video.status === "ready").length;
  const processingVideos = state.videos.filter((video) =>
    ["uploaded", "processing", "pending"].includes(video.status)
  ).length;
  const failedVideos = state.videos.filter((video) => video.status === "failed").length;
  const liveNow = state.liveSessions.filter((session) => session.status === "live").length;
  const gatewayStatus = state.health.find((item) => item.name === "api-gateway")?.status || "unknown";

  function persistAuth(auth: AuthResult) {
    setToken(auth.access_token);
    setRefreshToken(auth.refresh_token);
    setUser(auth.user);
    localStorage.setItem("admin_access_token", auth.access_token);
    localStorage.setItem("admin_refresh_token", auth.refresh_token);
    setMessage(`Signed in as ${auth.user.email}`);
  }

  function logout() {
    setToken("");
    setRefreshToken("");
    setUser(null);
    localStorage.removeItem("admin_access_token");
    localStorage.removeItem("admin_refresh_token");
    setMessage("Signed out locally.");
  }

  return (
    <main className="shell">
      <aside className="sidebar" aria-label="Primary navigation">
        <div className="brand">
          <div className="brandMark">
            <Gauge size={20} />
          </div>
          <div>
            <strong>AIOps Control</strong>
            <span>Video operations</span>
          </div>
        </div>

        <nav className="navList">
          <NavButton icon={<Activity size={17} />} label="Overview" tab="overview" activeTab={activeTab} onClick={setActiveTab} />
          <NavButton icon={<Video size={17} />} label="Videos" tab="videos" activeTab={activeTab} onClick={setActiveTab} />
          <NavButton icon={<Radio size={17} />} label="Live" tab="live" activeTab={activeTab} onClick={setActiveTab} />
          <NavButton icon={<Siren size={17} />} label="AIOps" tab="aiops" activeTab={activeTab} onClick={setActiveTab} />
          <NavButton icon={<KeyRound size={17} />} label="Account" tab="account" activeTab={activeTab} onClick={setActiveTab} />
        </nav>

        <div className="sidebarMeta">
          <span className={`serviceDot ${gatewayStatus}`} />
          <div>
            <strong>Gateway</strong>
            <span>{gatewayStatus}</span>
          </div>
        </div>
      </aside>

      <section className="workspace">
        <header className="topbar">
          <div>
            <p className="eyebrow">AIOps Video Platform / {tabTitle(activeTab)}</p>
            <h1>{tabTitle(activeTab)}</h1>
          </div>
          <div className="topbarActions">
            <label className="envSelect" aria-label="Environment">
              <span>Env</span>
              <select defaultValue="local">
                <option value="local">local</option>
                <option value="demo">demo</option>
                <option value="prod">prod</option>
              </select>
            </label>
            <div className="operator">
              <span>{user?.display_name || user?.email || "Not signed in"}</span>
              <strong>{token ? "Authenticated" : "Read-only"}</strong>
            </div>
            <button className="iconButton" type="button" onClick={() => refresh()} aria-label="Refresh">
              <RefreshCw size={18} className={state.loading ? "spin" : ""} />
            </button>
          </div>
        </header>

        {state.error ? <Banner tone="danger" icon={<AlertCircle size={17} />} text={state.error} /> : null}
        {message ? <Banner tone="success" icon={<CheckCircle2 size={17} />} text={message} /> : null}

        {activeTab === "overview" ? (
          <Overview
            health={state.health}
            feed={state.feed}
            readyVideos={readyVideos}
            processingVideos={processingVideos}
            failedVideos={failedVideos}
            liveNow={liveNow}
            gatewayStatus={gatewayStatus}
            updatedAt={state.updatedAt}
            loading={state.loading}
          />
        ) : null}

        {activeTab === "videos" ? (
          <Videos
            videos={filteredVideos}
            feed={state.feed}
            query={query}
            ownerFilter={ownerFilter}
            videoStatus={videoStatus}
            token={token}
            busyAction={busyAction}
            loading={state.loading}
            onQuery={setQuery}
            onOwnerFilter={setOwnerFilter}
            onStatus={setVideoStatus}
            onBusy={setBusyAction}
            onMessage={setMessage}
            onRefresh={() => refresh()}
          />
        ) : null}

        {activeTab === "live" ? (
          <Live
            sessions={state.liveSessions}
            token={token}
            busyAction={busyAction}
            onBusy={setBusyAction}
            onMessage={setMessage}
            onRefresh={() => refresh()}
          />
        ) : null}

        {activeTab === "aiops" ? (
          <AIOps token={token} busyAction={busyAction} onBusy={setBusyAction} onMessage={setMessage} />
        ) : null}

        {activeTab === "account" ? (
          <Account
            user={user}
            token={token}
            refreshToken={refreshToken}
            busyAction={busyAction}
            onBusy={setBusyAction}
            onAuth={persistAuth}
            onLogout={logout}
            onMessage={setMessage}
          />
        ) : null}
      </section>
    </main>
  );
}

function Overview({
  health,
  feed,
  readyVideos,
  processingVideos,
  failedVideos,
  liveNow,
  gatewayStatus,
  updatedAt,
  loading
}: {
  health: HealthCheck[];
  feed: FeedItem[];
  readyVideos: number;
  processingVideos: number;
  failedVideos: number;
  liveNow: number;
  gatewayStatus: string;
  updatedAt: string;
  loading: boolean;
}) {
  const displayedHealth: HealthCheck[] = health.length
    ? health
    : serviceNames.map((name) => ({ name, status: "unknown" }));

  return (
    <div className="stack">
      <section className="metrics">
        <Metric label="Gateway" value={gatewayStatus} detail="API edge" tone={gatewayStatus === "ready" ? "success" : "neutral"} />
        <Metric label="Ready videos" value={readyVideos.toString()} detail="Published to feed" />
        <Metric label="Processing" value={processingVideos.toString()} detail="Uploaded or queued" />
        <Metric label="Failures" value={failedVideos.toString()} detail="Need operator review" tone={failedVideos ? "danger" : "neutral"} />
        <Metric label="Live now" value={liveNow.toString()} detail="Active sessions" />
      </section>

      <section className="twoColumn">
        <Panel title="Service Readiness" subtitle={updatedAt ? `Last refresh ${updatedAt}` : "Gateway-backed status"}>
          <div className="healthGrid">
            {displayedHealth.map((item) => (
              <div className="healthItem" key={item.name}>
                <span className={`serviceDot ${item.status}`} />
                <div>
                  <strong>{item.name}</strong>
                  <span>{item.message || item.status}</span>
                </div>
              </div>
            ))}
          </div>
        </Panel>

        <Panel title="Viewer Feed" subtitle="Ready items currently visible">
          <div className="compactList">
            {feed.map((item) => (
              <div className="compactItem" key={item.video_id}>
                <FileVideo size={18} />
                <div>
                  <strong>{item.title || item.video_id}</strong>
                  <span>{item.like_count} likes · {item.comment_count} comments · {formatDuration(item.duration_ms)}</span>
                </div>
              </div>
            ))}
            {!feed.length ? <EmptyState label={loading ? "Loading feed..." : "No ready feed items."} /> : null}
          </div>
        </Panel>
      </section>

      <Panel title="Operational Queue" subtitle="Current admin focus">
        <div className="queueGrid">
          <QueueItem label="Incident backlog" value="0 open" state="ready" />
          <QueueItem label="Worker retries" value={`${processingVideos} active`} state={processingVideos ? "warning" : "ready"} />
          <QueueItem label="Video failures" value={`${failedVideos} failed`} state={failedVideos ? "danger" : "ready"} />
          <QueueItem label="RCA reports" value="pending backend" state="unknown" />
        </div>
      </Panel>
    </div>
  );
}

function Videos({
  videos,
  feed,
  query,
  ownerFilter,
  videoStatus,
  token,
  busyAction,
  loading,
  onQuery,
  onOwnerFilter,
  onStatus,
  onBusy,
  onMessage,
  onRefresh
}: {
  videos: VideoItem[];
  feed: FeedItem[];
  query: string;
  ownerFilter: string;
  videoStatus: string;
  token: string;
  busyAction: string;
  loading: boolean;
  onQuery: (value: string) => void;
  onOwnerFilter: (value: string) => void;
  onStatus: (value: string) => void;
  onBusy: (value: string) => void;
  onMessage: (value: string) => void;
  onRefresh: () => Promise<void>;
}) {
  const [uploadOpen, setUploadOpen] = useState(false);

  async function submitUpload(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      onMessage("Sign in before creating upload requests.");
      return;
    }
    const form = new FormData(event.currentTarget);
    const file = form.get("file");
    if (!(file instanceof File) || file.size === 0) {
      onMessage("Choose a video file first.");
      return;
    }
    onBusy("upload");
    try {
      const checksum = await sha256Hex(file);
      const intent = await createUploadRequest(
        {
          title: String(form.get("title") || file.name),
          description: String(form.get("description") || ""),
          visibility: String(form.get("visibility") || "public"),
          content_type: file.type || "video/mp4",
          size_bytes: file.size,
          checksum_sha256: checksum
        },
        token
      );
      await uploadObject(intent.upload_request.upload_url, file);
      await confirmUploaded(
        intent.video.id,
        {
          upload_request_id: intent.upload_request.id,
          size_bytes: file.size,
          checksum_sha256: checksum
        },
        token
      );
      event.currentTarget.reset();
      setUploadOpen(false);
      onMessage(`Upload confirmed for ${intent.video.id}. Worker processing should start next.`);
      await onRefresh();
    } catch (error) {
      onMessage(errorMessage(error));
    } finally {
      onBusy("");
    }
  }

  return (
    <div className="stack">
      <section className="toolbar">
        <div className="searchBox">
          <Search size={16} />
          <input value={query} onChange={(event) => onQuery(event.target.value)} placeholder="Search title, owner, status" />
        </div>
        <div className="searchBox ownerBox">
          <Search size={16} />
          <input value={ownerFilter} onChange={(event) => onOwnerFilter(event.target.value)} placeholder="Owner ID" />
        </div>
        <label className="selectBox">
          <ListFilter size={16} />
          <select value={videoStatus} onChange={(event) => onStatus(event.target.value)} aria-label="Filter videos by status">
            <option value="">All statuses</option>
            <option value="pending">Pending</option>
            <option value="uploaded">Uploaded</option>
            <option value="processing">Processing</option>
            <option value="ready">Ready</option>
            <option value="failed">Failed</option>
          </select>
        </label>
        <button className="primaryButton toolbarAction" type="button" disabled={!token} onClick={() => setUploadOpen(true)}>
          <UploadCloud size={16} />
          Upload
        </button>
      </section>

      <section className="sideBySide">
        <Panel title="Feed Read Model" subtitle="Items emitted from ready-video events">
          <div className="compactList">
            {feed.slice(0, 6).map((item) => (
              <div className="compactItem" key={item.video_id}>
                <Video size={18} />
                <div>
                  <strong>{item.title || item.video_id}</strong>
                  <span>{item.playback_object_key || "playback object pending"}</span>
                </div>
              </div>
            ))}
            {!feed.length ? <EmptyState label="No ready feed items yet." /> : null}
          </div>
        </Panel>
      </section>

      <Panel title="Video Pipeline" subtitle="Canonical video metadata from video-service">
        <DataTable
          columns={["Video", "Owner", "Status", "Visibility", "Asset", "Updated"]}
          empty={loading ? "Loading videos..." : "No videos match the current filter."}
          rows={videos.map((video) => [
            <CellTitle key="video" title={video.title || video.id} detail={video.id} />,
            video.owner_id || "-",
            <StatusPill key="status" status={video.status} />,
            video.visibility || "-",
            video.processed_object_key || video.raw_object_key || "-",
            formatDate(video.updated_at)
          ])}
        />
      </Panel>

      <Drawer title="Upload video" open={uploadOpen} onClose={() => setUploadOpen(false)}>
        <form className="formGrid drawerForm" onSubmit={submitUpload}>
          <Field label="Title" name="title" placeholder="Kubernetes incident demo clip" required />
          <label className="field">
            <span>Visibility</span>
            <select name="visibility" defaultValue="public">
              <option value="public">Public</option>
              <option value="private">Private</option>
            </select>
          </label>
          <label className="field span2">
            <span>Description</span>
            <textarea name="description" rows={3} placeholder="Short operator note" />
          </label>
          <label className="field span2">
            <span>File</span>
            <input name="file" type="file" accept="video/*" required />
          </label>
          <button className="primaryButton span2" disabled={!token || busyAction === "upload"} type="submit">
            <UploadCloud size={16} />
            {busyAction === "upload" ? "Uploading..." : "Upload and confirm"}
          </button>
        </form>
      </Drawer>
    </div>
  );
}

function Live({
  sessions,
  token,
  busyAction,
  onBusy,
  onMessage,
  onRefresh
}: {
  sessions: LiveSession[];
  token: string;
  busyAction: string;
  onBusy: (value: string) => void;
  onMessage: (value: string) => void;
  onRefresh: () => Promise<void>;
}) {
  const [createOpen, setCreateOpen] = useState(false);

  async function submitLive(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      onMessage("Sign in before creating a live session.");
      return;
    }
    const form = new FormData(event.currentTarget);
    onBusy("live");
    try {
      const scheduled = String(form.get("scheduled_at") || "");
      const session = await createLiveSession(
        {
          title: String(form.get("title") || ""),
          description: String(form.get("description") || ""),
          scheduled_at: scheduled ? new Date(scheduled).toISOString() : undefined
        },
        token
      );
      event.currentTarget.reset();
      setCreateOpen(false);
      onMessage(`Live session created. Stream key: ${session.stream_key || "hidden after creation"}`);
      await onRefresh();
    } catch (error) {
      onMessage(errorMessage(error));
    } finally {
      onBusy("");
    }
  }

  async function transition(id: string, action: "start" | "end") {
    if (!token) {
      onMessage("Sign in before changing live state.");
      return;
    }
    onBusy(`${action}-${id}`);
    try {
      await transitionLiveSession(id, action, token);
      onMessage(`Live session ${action} accepted.`);
      await onRefresh();
    } catch (error) {
      onMessage(errorMessage(error));
    } finally {
      onBusy("");
    }
  }

  return (
    <div className="stack">
      <section className="toolbar">
        <button className="primaryButton toolbarAction" type="button" disabled={!token} onClick={() => setCreateOpen(true)}>
          <Radio size={16} />
          New session
        </button>
      </section>

      <section className="sideBySide">
        <Panel title="Stream Operations" subtitle="Start and end lifecycle states">
          <div className="compactList">
            {sessions.slice(0, 5).map((session) => (
              <div className="opsItem" key={session.id}>
                <div>
                  <strong>{session.title || session.id}</strong>
                  <span>{session.playback_url || session.creator_id}</span>
                </div>
                <div className="rowActions">
                  <StatusPill status={session.status} />
                  <button className="miniButton" disabled={!token || busyAction === `start-${session.id}`} onClick={() => transition(session.id, "start")} type="button">
                    Start
                  </button>
                  <button className="miniButton" disabled={!token || busyAction === `end-${session.id}`} onClick={() => transition(session.id, "end")} type="button">
                    End
                  </button>
                </div>
              </div>
            ))}
            {!sessions.length ? <EmptyState label="No live sessions yet." /> : null}
          </div>
        </Panel>
      </section>

      <Panel title="Live Sessions" subtitle="Live-service state table">
        <DataTable
          columns={["Session", "Creator", "Status", "Ingest", "Playback", "Updated"]}
          empty="No live sessions found."
          rows={sessions.map((session) => [
            <CellTitle key="session" title={session.title || session.id} detail={session.id} />,
            session.creator_id || "-",
            <StatusPill key="status" status={session.status} />,
            session.ingest_url || "-",
            session.playback_url || "-",
            formatDate(session.updated_at)
          ])}
        />
      </Panel>

      <Drawer title="Create live session" open={createOpen} onClose={() => setCreateOpen(false)}>
        <form className="formGrid drawerForm" onSubmit={submitLive}>
          <Field label="Title" name="title" placeholder="Ops livestream" required />
          <Field label="Scheduled at" name="scheduled_at" type="datetime-local" />
          <label className="field span2">
            <span>Description</span>
            <textarea name="description" rows={3} placeholder="What this stream is about" />
          </label>
          <button className="primaryButton span2" disabled={!token || busyAction === "live"} type="submit">
            <Radio size={16} />
            {busyAction === "live" ? "Creating..." : "Create session"}
          </button>
        </form>
      </Drawer>
    </div>
  );
}

function AIOps({
  token,
  busyAction,
  onBusy,
  onMessage
}: {
  token: string;
  busyAction: string;
  onBusy: (value: string) => void;
  onMessage: (value: string) => void;
}) {
  async function submitIncident(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (!token) {
      onMessage("Sign in before creating incidents.");
      return;
    }
    const form = new FormData(event.currentTarget);
    onBusy("incident");
    try {
      const incident = await createIncident(
        {
          service: String(form.get("service") || "video-service"),
          namespace: String(form.get("namespace") || "app-demo"),
          symptom: String(form.get("symptom") || ""),
          severity: String(form.get("severity") || "medium"),
          started_at: new Date(String(form.get("started_at") || Date.now())).toISOString(),
          time_window: String(form.get("time_window") || "30m")
        },
        token
      );
      event.currentTarget.reset();
      onMessage(`Incident created: ${incident.id}`);
    } catch (error) {
      onMessage(`${errorMessage(error)} AIOps incident routes are still backend placeholders.`);
    } finally {
      onBusy("");
    }
  }

  return (
    <div className="stack">
      <section className="twoColumn">
        <Panel title="Open Incident" subtitle="Create an RCA work item for aiops-service">
          <form className="formGrid" onSubmit={submitIncident}>
            <label className="field">
              <span>Service</span>
              <select name="service" defaultValue="video-service">
                <option value="video-service">video-service</option>
                <option value="media-worker">media-worker</option>
                <option value="feed-social-service">feed-social-service</option>
                <option value="live-service">live-service</option>
                <option value="api-gateway">api-gateway</option>
              </select>
            </label>
            <Field label="Namespace" name="namespace" defaultValue="app-demo" />
            <label className="field">
              <span>Severity</span>
              <select name="severity" defaultValue="high">
                <option value="low">Low</option>
                <option value="medium">Medium</option>
                <option value="high">High</option>
                <option value="critical">Critical</option>
              </select>
            </label>
            <Field label="Started at" name="started_at" type="datetime-local" />
            <Field label="Time window" name="time_window" defaultValue="30m" />
            <label className="field span2">
              <span>Symptom</span>
              <textarea name="symptom" rows={4} placeholder="CrashLoopBackOff after media-worker rollout" required />
            </label>
            <button className="primaryButton span2" disabled={!token || busyAction === "incident"} type="submit">
              <Siren size={17} />
              {busyAction === "incident" ? "Creating..." : "Create incident"}
            </button>
          </form>
        </Panel>

        <Panel title="RCA Readiness" subtitle="What the thesis demo still needs">
          <div className="timeline">
            <TimelineItem title="Evidence collectors" detail="Prometheus, Loki, Kubernetes, Argo CD and Git evidence." done={false} />
            <TimelineItem title="Agent analysis" detail="Planner, metric, log and deployment agents with cited evidence." done={false} />
            <TimelineItem title="RCA report" detail="Root cause candidates, confidence and GitOps-safe actions." done={false} />
          </div>
        </Panel>
      </section>
    </div>
  );
}

function Account({
  user,
  token,
  refreshToken,
  busyAction,
  onBusy,
  onAuth,
  onLogout,
  onMessage
}: {
  user: User | null;
  token: string;
  refreshToken: string;
  busyAction: string;
  onBusy: (value: string) => void;
  onAuth: (auth: AuthResult) => void;
  onLogout: () => void;
  onMessage: (value: string) => void;
}) {
  async function submitLogin(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onBusy("login");
    try {
      onAuth(await login(String(form.get("email") || ""), String(form.get("password") || "")));
      event.currentTarget.reset();
    } catch (error) {
      onMessage(errorMessage(error));
    } finally {
      onBusy("");
    }
  }

  async function submitRegister(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    onBusy("register");
    try {
      const created = await register({
        email: String(form.get("email") || ""),
        username: String(form.get("username") || ""),
        display_name: String(form.get("display_name") || ""),
        password: String(form.get("password") || "")
      });
      event.currentTarget.reset();
      onMessage(`Created user ${created.email}. You can sign in now.`);
    } catch (error) {
      onMessage(errorMessage(error));
    } finally {
      onBusy("");
    }
  }

  return (
    <div className="stack">
      <section className="twoColumn">
        <Panel title="Sign In" subtitle="Gateway will forward trusted user context">
          <form className="formGrid" onSubmit={submitLogin}>
            <Field label="Email" name="email" type="email" placeholder="admin@example.com" required />
            <Field label="Password" name="password" type="password" required />
            <button className="primaryButton span2" disabled={busyAction === "login"} type="submit">
              <KeyRound size={17} />
              {busyAction === "login" ? "Signing in..." : "Sign in"}
            </button>
          </form>
        </Panel>

        <Panel title="Register Operator" subtitle="Create a local demo account">
          <form className="formGrid" onSubmit={submitRegister}>
            <Field label="Email" name="email" type="email" required />
            <Field label="Username" name="username" required />
            <Field label="Display name" name="display_name" required />
            <Field label="Password" name="password" type="password" required />
            <button className="secondaryButton span2" disabled={busyAction === "register"} type="submit">
              <UserPlus size={17} />
              {busyAction === "register" ? "Creating..." : "Create account"}
            </button>
          </form>
        </Panel>
      </section>

      <Panel title="Session" subtitle="Browser-local token state">
        <div className="sessionPanel">
          <div>
            <strong>{user?.email || "No active user"}</strong>
            <span>{user?.id || "Sign in to unlock admin actions."}</span>
          </div>
          <div className="tokenPreview">
            <span>Access token</span>
            <code>{token ? `${token.slice(0, 24)}...` : "empty"}</code>
          </div>
          <div className="tokenPreview">
            <span>Refresh token</span>
            <code>{refreshToken ? `${refreshToken.slice(0, 18)}...` : "empty"}</code>
          </div>
          <button className="dangerButton" type="button" onClick={onLogout} disabled={!token}>
            <LogOut size={17} />
            Sign out
          </button>
        </div>
      </Panel>
    </div>
  );
}

function NavButton({
  icon,
  label,
  tab,
  activeTab,
  onClick
}: {
  icon: ReactNode;
  label: string;
  tab: Tab;
  activeTab: Tab;
  onClick: (tab: Tab) => void;
}) {
  return (
    <button className={activeTab === tab ? "active" : ""} type="button" onClick={() => onClick(tab)}>
      {icon}
      {label}
    </button>
  );
}

function Metric({
  label,
  value,
  detail,
  tone = "neutral"
}: {
  label: string;
  value: string;
  detail: string;
  tone?: "neutral" | "danger" | "success";
}) {
  return (
    <article className={`metric ${tone}`}>
      <span>{label}</span>
      <strong>{value}</strong>
      <p>{detail}</p>
    </article>
  );
}

function QueueItem({ label, value, state }: { label: string; value: string; state: "ready" | "warning" | "danger" | "unknown" }) {
  return (
    <div className="queueItem">
      <span className={`serviceDot ${state}`} />
      <span>{label}</span>
      <strong>{value}</strong>
    </div>
  );
}

function Panel({ title, subtitle, children }: { title: string; subtitle: string; children: ReactNode }) {
  return (
    <section className="panel">
      <header className="panelHeader">
        <div>
          <h2>{title}</h2>
          <p>{subtitle}</p>
        </div>
      </header>
      {children}
    </section>
  );
}

function Field({
  label,
  name,
  type = "text",
  placeholder,
  defaultValue,
  required
}: {
  label: string;
  name: string;
  type?: string;
  placeholder?: string;
  defaultValue?: string;
  required?: boolean;
}) {
  return (
    <label className="field">
      <span>{label}</span>
      <input name={name} type={type} placeholder={placeholder} defaultValue={defaultValue} required={required} />
    </label>
  );
}

function DataTable({ columns, rows, empty }: { columns: string[]; rows: ReactNode[][]; empty: string }) {
  return (
    <div className="tableWrap">
      <table>
        <thead>
          <tr>
            {columns.map((column) => (
              <th key={column}>{column}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={index}>
              {row.map((cell, cellIndex) => (
                <td key={cellIndex}>{cell}</td>
              ))}
            </tr>
          ))}
          {!rows.length ? (
            <tr>
              <td className="emptyCell" colSpan={columns.length}>
                {empty}
              </td>
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}

function CellTitle({ title, detail }: { title: string; detail: string }) {
  return (
    <div className="cellTitle">
      <strong>{title}</strong>
      <span>{detail}</span>
    </div>
  );
}

function StatusPill({ status }: { status: string }) {
  return <span className={`status ${status || "unknown"}`}>{status || "unknown"}</span>;
}

function Drawer({
  title,
  open,
  onClose,
  children
}: {
  title: string;
  open: boolean;
  onClose: () => void;
  children: ReactNode;
}) {
  if (!open) {
    return null;
  }
  return (
    <div className="drawerLayer" role="dialog" aria-modal="true" aria-label={title}>
      <button className="drawerScrim" type="button" aria-label="Close drawer" onClick={onClose} />
      <aside className="drawer">
        <header className="drawerHeader">
          <h2>{title}</h2>
          <button className="iconButton compact" type="button" aria-label="Close" onClick={onClose}>
            <X size={16} />
          </button>
        </header>
        {children}
      </aside>
    </div>
  );
}

function Banner({ tone, icon, text }: { tone: "success" | "danger"; icon: ReactNode; text: string }) {
  return (
    <div className={`banner ${tone}`} role="status">
      {icon}
      <span>{text}</span>
    </div>
  );
}

function EmptyState({ label }: { label: string }) {
  return <div className="emptyState">{label}</div>;
}

function TimelineItem({ title, detail, done }: { title: string; detail: string; done: boolean }) {
  return (
    <div className="timelineItem">
      {done ? <CheckCircle2 size={18} /> : <Clock3 size={18} />}
      <div>
        <strong>{title}</strong>
        <span>{detail}</span>
      </div>
    </div>
  );
}

function tabTitle(tab: Tab) {
  switch (tab) {
    case "videos":
      return "Video Pipeline";
    case "live":
      return "Live Operations";
    case "aiops":
      return "Incident RCA";
    case "account":
      return "Operator Account";
    default:
      return "Operations Overview";
  }
}

function formatDuration(value?: number) {
  if (!value) {
    return "-";
  }
  const seconds = Math.round(value / 1000);
  const minutes = Math.floor(seconds / 60);
  const rest = seconds % 60;
  return `${minutes}:${rest.toString().padStart(2, "0")}`;
}

function formatDate(value?: string) {
  if (!value) {
    return "-";
  }
  return new Intl.DateTimeFormat("en", {
    month: "short",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit"
  }).format(new Date(value));
}

function errorMessage(error: unknown) {
  if (error instanceof ApiError) {
    return `${error.message} (${error.status})`;
  }
  if (error instanceof Error) {
    return error.message;
  }
  return "Unexpected error.";
}
