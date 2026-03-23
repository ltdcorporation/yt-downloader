import { detectPlatform } from "./utils";

const API_BASE_URL =
  process.env.NEXT_PUBLIC_API_BASE_URL ||
  process.env.NEXT_PUBLIC_API_URL ||
  "/api";

export interface ResolveFormat {
  id: string;
  quality: string;
  container: string;
  type: "mp4" | "mp3" | string;
  progressive: boolean;
  url?: string;
  filesize?: number;
}

export interface ResolveResponse {
  title: string;
  thumbnail: string;
  duration_seconds: number;
  formats: ResolveFormat[];
  medias?: {
    id: string;
    type: "video" | "image";
    url: string;
    thumbnail?: string;
    quality?: string;
  }[];
  kind?: "video" | "image" | "carousel";
  author?: string;
  views?: string;
  likes?: string;
  shares?: string;
}

export interface CreateMp3JobResponse {
  job_id: string;
  status: string;
}

export interface JobStatusResponse {
  id: string;
  status: string;
  download_url?: string;
  error?: string;
  created_at?: string;
  updated_at?: string;
}

export type HistoryPlatform = "youtube" | "tiktok" | "instagram" | "x";

export type HistoryAttemptStatus =
  | "queued"
  | "processing"
  | "done"
  | "failed"
  | "expired";

export type HistoryRequestKind = "mp3" | "mp4" | "image";

export interface HistoryLatestAttempt {
  id: string;
  request_kind: HistoryRequestKind;
  status: HistoryAttemptStatus;
  format_id?: string;
  quality_label?: string;
  size_bytes?: number | null;
  download_url?: string;
  expires_at?: string | null;
  created_at?: string;
}

export interface HistoryListItem {
  id: string;
  title: string;
  thumbnail_url: string;
  platform: HistoryPlatform;
  source_url: string;
  last_attempt_at: string;
  latest_attempt?: HistoryLatestAttempt | null;
}

export interface HistoryListResponse {
  items: HistoryListItem[];
  page: {
    next_cursor?: string | null;
    has_more: boolean;
    limit: number;
  };
}

export interface HistoryStatsResponse {
  total_items: number;
  total_attempts: number;
  success_count: number;
  failed_count: number;
  total_bytes_downloaded: number;
  this_month_attempts: number;
}

export interface HistoryRedownloadResponse {
  mode: "direct" | "queued";
  download_url?: string;
  job_id?: string;
  status?: string;
}

export interface AuthUser {
  id: string;
  full_name: string;
  email: string;
  created_at: string;
}

export interface AuthResponse {
  user: AuthUser;
  access_token: string;
  token_type: string;
  expires_at: string;
}

export interface AuthMeResponse {
  user: AuthUser;
  expires_at: string;
}

export type SettingsQuality = "4k" | "1080p" | "720p" | "480p";

export interface SettingsSnapshotResponse {
  settings: {
    preferences: {
      default_quality: SettingsQuality;
      auto_trim_silence: boolean;
      thumbnail_generation: boolean;
    };
    notifications: {
      email: {
        processing: boolean;
        storage: boolean;
        summary: boolean;
      };
    };
  };
  meta: {
    version: number;
    updated_at: string;
  };
}

export interface SettingsPatchRequest {
  settings: {
    preferences?: {
      default_quality?: SettingsQuality;
      auto_trim_silence?: boolean;
      thumbnail_generation?: boolean;
    };
    notifications?: {
      email?: {
        processing?: boolean;
        storage?: boolean;
        summary?: boolean;
      };
    };
  };
  meta: {
    version: number;
  };
}

export interface ProfileResponse {
  profile: AuthUser;
}

export class APIError extends Error {
  code?: string;

  constructor(message: string, code?: string) {
    super(message);
    this.name = "APIError";
    this.code = code;
  }
}

async function parseJSONResponse<T>(response: Response): Promise<T> {
  const contentType = response.headers.get("content-type") || "";
  if (!contentType.toLowerCase().includes("application/json")) {
    throw new APIError("Unexpected response format from server");
  }
  return response.json() as Promise<T>;
}

export async function fetcher<T>(
  endpoint: string,
  options?: RequestInit,
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    credentials: "include",
    ...options,
    headers: {
      Accept: "application/json",
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const errorPayload: { error?: string; message?: string; code?: string } =
      await response
        .json()
        .catch(() => ({ error: `HTTP ${response.status}` }));

    const message =
      errorPayload?.error || errorPayload?.message || `HTTP ${response.status}`;

    throw new APIError(message, errorPayload?.code);
  }

  return parseJSONResponse<T>(response);
}

export const api = {
  health: () =>
    fetcher<{ ok: boolean; service: string; time: string }>("/healthz"),

  register: (payload: {
    fullName: string;
    email: string;
    password: string;
    keepLoggedIn?: boolean;
  }) =>
    fetcher<AuthResponse>("/v1/auth/register", {
      method: "POST",
      body: JSON.stringify({
        full_name: payload.fullName,
        email: payload.email,
        password: payload.password,
        keep_logged_in: payload.keepLoggedIn ?? false,
      }),
    }),

  login: (payload: {
    email: string;
    password: string;
    keepLoggedIn?: boolean;
  }) =>
    fetcher<AuthResponse>("/v1/auth/login", {
      method: "POST",
      body: JSON.stringify({
        email: payload.email,
        password: payload.password,
        keep_logged_in: payload.keepLoggedIn ?? false,
      }),
    }),

  loginWithGoogle: (payload: { idToken: string; keepLoggedIn?: boolean }) =>
    fetcher<AuthResponse>("/v1/auth/google", {
      method: "POST",
      body: JSON.stringify({
        id_token: payload.idToken,
        keep_logged_in: payload.keepLoggedIn ?? false,
      }),
    }),

  me: () => fetcher<AuthMeResponse>("/v1/auth/me"),

  logout: () =>
    fetcher<{ ok: boolean }>("/v1/auth/logout", {
      method: "POST",
    }),

  profile: () => fetcher<ProfileResponse>("/v1/profile"),

  updateProfile: (payload: { fullName: string }) =>
    fetcher<ProfileResponse>("/v1/profile", {
      method: "PATCH",
      body: JSON.stringify({
        profile: {
          full_name: payload.fullName,
        },
      }),
    }),

  getSettings: () => fetcher<SettingsSnapshotResponse>("/v1/settings"),

  updateSettings: (payload: SettingsPatchRequest) =>
    fetcher<SettingsSnapshotResponse>("/v1/settings", {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  resolve: (url: string) => {
    const platform = detectPlatform(url);
    if (platform === "unknown") {
      throw new Error("Unsupported or invalid social media URL.");
    }
    return fetcher<ResolveResponse>(`/v1/${platform}/resolve`, {
      method: "POST",
      body: JSON.stringify({ url }),
    });
  },

  createMp3Job: (url: string) =>
    fetcher<CreateMp3JobResponse>("/v1/jobs/mp3", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),

  getJobStatus: (jobId: string) =>
    fetcher<JobStatusResponse>(`/v1/jobs/${jobId}`),

  historyList: (params?: {
    limit?: number;
    cursor?: string;
    platform?: HistoryPlatform;
    q?: string;
    status?: HistoryAttemptStatus;
  }) => {
    const query = new URLSearchParams();

    if (typeof params?.limit === "number" && params.limit > 0) {
      query.set("limit", String(params.limit));
    }
    if (params?.cursor) {
      query.set("cursor", params.cursor);
    }
    if (params?.platform) {
      query.set("platform", params.platform);
    }
    if (params?.q) {
      query.set("q", params.q);
    }
    if (params?.status) {
      query.set("status", params.status);
    }

    const suffix = query.toString();
    return fetcher<HistoryListResponse>(
      suffix ? `/v1/history?${suffix}` : "/v1/history",
    );
  },

  historyStats: () => fetcher<HistoryStatsResponse>("/v1/history/stats"),

  historyDelete: (id: string) =>
    fetcher<{ ok: boolean }>(`/v1/history/${encodeURIComponent(id)}`, {
      method: "DELETE",
    }),

  historyRedownload: (
    id: string,
    payload?: {
      requestKind?: HistoryRequestKind;
      formatId?: string;
    },
  ) => {
    const body: Record<string, string> = {};
    if (payload?.requestKind) {
      body.request_kind = payload.requestKind;
    }
    if (payload?.formatId) {
      body.format_id = payload.formatId;
    }

    return fetcher<HistoryRedownloadResponse>(
      `/v1/history/${encodeURIComponent(id)}/redownload`,
      {
        method: "POST",
        ...(Object.keys(body).length > 0
          ? { body: JSON.stringify(body) }
          : undefined),
      },
    );
  },

  getMp4DownloadUrl: (url: string, formatId: string) =>
    `${API_BASE_URL}/v1/download/mp4?url=${encodeURIComponent(url)}&format_id=${encodeURIComponent(formatId)}`,
};
