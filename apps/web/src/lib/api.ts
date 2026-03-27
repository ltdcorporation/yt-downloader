import { detectPlatform } from "./utils";
import { readAdminAuthCredentials } from "./auth-session";

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

export interface ResolveHeatmapPoint {
  start_time: number;
  end_time: number;
  value: number;
}

export interface ResolveHeatmapMeta {
  available: boolean;
  bins: number;
  algorithm_version: string;
}

export interface ResolveResponse {
  title: string;
  thumbnail: string;
  duration_seconds: number;
  formats: ResolveFormat[];
  heatmap?: ResolveHeatmapPoint[];
  key_moments?: number[];
  heatmap_meta?: ResolveHeatmapMeta;
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

export type VideoCutMode = "manual" | "heatmap";

export interface CreateVideoCutJobRequest {
  url: string;
  formatId: string;
  cutMode: VideoCutMode;
  manual?: {
    startSec: number;
    endSec: number;
  };
  heatmap?: {
    targetSec?: number;
    windowSec?: number;
  };
}

export interface CreateVideoCutJobResponse {
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
  | "resolved"
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

export type UserRole = "admin" | "user";
export type UserPlan = "free" | "daily" | "weekly" | "monthly";

export interface AuthUser {
  id: string;
  full_name: string;
  email: string;
  avatar_url?: string;
  role: UserRole;
  plan: UserPlan;
  plan_expires_at?: string;
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

export interface AdminSettingsSnapshotResponse {
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
    updated_by_user_id?: string;
  };
}

export interface AdminSettingsPatchRequest {
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

export type MaintenanceServiceKey =
  | "api_gateway"
  | "primary_database"
  | "worker_nodes";

export type MaintenanceServiceStatus =
  | "active"
  | "maintenance"
  | "scaling"
  | "refreshing";

export interface MaintenanceServiceItem {
  key: MaintenanceServiceKey;
  name: string;
  status: MaintenanceServiceStatus;
  enabled: boolean;
}

export interface MaintenanceSnapshotResponse {
  maintenance: {
    enabled: boolean;
    estimated_downtime: string;
    public_message: string;
    services: MaintenanceServiceItem[];
  };
  meta: {
    version: number;
    updated_at: string;
    updated_by_user_id?: string;
  };
}

export interface MaintenancePatchRequest {
  maintenance: {
    enabled?: boolean;
    estimated_downtime?: string;
    public_message?: string;
    services?: Array<{
      key: MaintenanceServiceKey;
      status?: MaintenanceServiceStatus;
      enabled?: boolean;
    }>;
  };
  meta: {
    version: number;
  };
}

export type SubscriptionStatus =
  | "active"
  | "inactive"
  | "expired"
  | "cancel_scheduled";

export interface SubscriptionBenefit {
  id: string;
  label: string;
}

export interface SubscriptionSummary {
  plan: UserPlan;
  status: SubscriptionStatus;
  interval: string;
  price_cents: number;
  currency: string;
  plan_expires_at?: string;
  next_billing_at?: string;
  cancel_at_period_end: boolean;
  benefits: SubscriptionBenefit[];
}

export interface BillingPaymentMethod {
  brand: string;
  last4: string;
  exp_month: number;
  exp_year: number;
  updated_at?: string;
}

export interface SubscriptionDashboardResponse {
  subscription: SubscriptionSummary;
  payment_method: BillingPaymentMethod;
}

export type BillingInvoiceStatus = "paid" | "pending" | "failed";

export interface BillingInvoice {
  id: string;
  issued_at: string;
  amount_cents: number;
  amount: string;
  currency: string;
  status: BillingInvoiceStatus;
  receipt_url?: string;
  period_start?: string;
  period_end?: string;
}

export interface BillingHistoryResponse {
  items: BillingInvoice[];
  page: {
    total: number;
    limit: number;
    offset: number;
  };
}

export interface AdminUsersStats {
  total_users: number;
  admin_users: number;
  member_users: number;
  free_users: number;
  daily_users: number;
  weekly_users: number;
  monthly_users: number;
  active_paid_users: number;
}

export interface AdminUsersStatsResponse {
  stats: AdminUsersStats;
}

export type JobStatus = "queued" | "processing" | "done" | "failed";

export interface AdminJobRecord {
  id: string;
  status: JobStatus | string;
  input_url: string;
  output_kind: string;
  output_key?: string;
  title?: string;
  error?: string;
  download_url?: string;
  created_at: string;
  updated_at: string;
  expires_at?: string;
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

async function parseAPIError(response: Response): Promise<APIError> {
  const errorPayload: { error?: string; message?: string; code?: string } =
    await response
      .json()
      .catch(() => ({ error: `HTTP ${response.status}` }));

  const message =
    errorPayload?.error || errorPayload?.message || `HTTP ${response.status}`;

  return new APIError(message, errorPayload?.code);
}

export async function fetcher<T>(
  endpoint: string,
  options?: RequestInit,
): Promise<T> {
  const headers = new Headers({
    Accept: "application/json",
    "Content-Type": "application/json",
  });

  // Merge headers from options
  if (options?.headers) {
    const customHeaders = new Headers(options.headers);
    customHeaders.forEach((value, key) => {
      headers.set(key, value);
    });
  }

  // Automatically attach admin auth if present in sessionStorage
  if (typeof window !== "undefined") {
    const adminAuth = readAdminAuthCredentials();
    if (adminAuth && !headers.has("Authorization")) {
      headers.set("Authorization", `Basic ${adminAuth}`);
    }
  }

  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    credentials: "include",
    ...options,
    headers,
  });

  if (!response.ok) {
    throw await parseAPIError(response);
  }

  return parseJSONResponse<T>(response);
}

export async function fetcherWithAuth<T>(
  endpoint: string,
  auth: { user: string; pass: string },
  options?: RequestInit,
): Promise<T> {
  const credentials = btoa(`${auth.user}:${auth.pass}`);
  
  // We explicitly pass the Authorization header in the options
  return fetcher<T>(endpoint, {
    ...options,
    headers: {
      ...options?.headers,
      "Authorization": `Basic ${credentials}`,
    },
  });
}

async function fetcherFormData<T>(
  endpoint: string,
  formData: FormData,
  options?: Omit<RequestInit, "body">,
): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    credentials: "include",
    ...options,
    method: options?.method || "POST",
    body: formData,
    headers: {
      Accept: "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    throw await parseAPIError(response);
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

  loginAdmin: (user: string, pass: string) =>
    fetcherWithAuth<AuthMeResponse>("/v1/auth/me", { user, pass }),

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

  uploadProfileAvatar: (file: File) => {
    const formData = new FormData();
    formData.append("avatar", file);
    return fetcherFormData<ProfileResponse>("/v1/profile/avatar", formData, {
      method: "POST",
    });
  },

  removeProfileAvatar: () =>
    fetcher<ProfileResponse>("/v1/profile/avatar", {
      method: "DELETE",
    }),

  getSettings: () => fetcher<SettingsSnapshotResponse>("/v1/settings"),

  updateSettings: (payload: SettingsPatchRequest) =>
    fetcher<SettingsSnapshotResponse>("/v1/settings", {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  getAdminSettings: () =>
    fetcher<AdminSettingsSnapshotResponse>("/v1/admin/settings"),

  updateAdminSettings: (payload: AdminSettingsPatchRequest) =>
    fetcher<AdminSettingsSnapshotResponse>("/v1/admin/settings", {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  getMaintenance: () =>
    fetcher<MaintenanceSnapshotResponse>("/v1/maintenance"),

  getAdminMaintenance: () =>
    fetcher<MaintenanceSnapshotResponse>("/v1/admin/maintenance"),

  updateAdminMaintenance: (payload: MaintenancePatchRequest) =>
    fetcher<MaintenanceSnapshotResponse>("/v1/admin/maintenance", {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  getSubscription: () =>
    fetcher<SubscriptionDashboardResponse>("/v1/subscription"),

  updateSubscription: (payload: { plan: UserPlan }) =>
    fetcher<SubscriptionDashboardResponse>("/v1/subscription", {
      method: "PATCH",
      body: JSON.stringify({
        subscription: {
          plan: payload.plan,
        },
      }),
    }),

  cancelSubscription: (payload?: { immediate?: boolean }) =>
    fetcher<SubscriptionDashboardResponse>("/v1/subscription/cancel", {
      method: "POST",
      body: JSON.stringify({
        immediate: payload?.immediate ?? false,
      }),
    }),

  reactivateSubscription: () =>
    fetcher<SubscriptionDashboardResponse>("/v1/subscription/reactivate", {
      method: "POST",
    }),

  listBillingHistory: (params?: { limit?: number; offset?: number }) => {
    const query = new URLSearchParams();

    if (typeof params?.limit === "number" && params.limit > 0) {
      query.set("limit", String(params.limit));
    }
    if (typeof params?.offset === "number" && params.offset >= 0) {
      query.set("offset", String(params.offset));
    }

    const suffix = query.toString();
    return fetcher<BillingHistoryResponse>(
      suffix ? `/v1/billing/history?${suffix}` : "/v1/billing/history",
    );
  },

  getBillingInvoice: (invoiceID: string) =>
    fetcher<BillingInvoice>(`/v1/billing/invoices/${encodeURIComponent(invoiceID)}`),

  getBillingReceiptUrl: (invoiceID: string) =>
    `${API_BASE_URL}/v1/billing/invoices/${encodeURIComponent(invoiceID)}/receipt`,

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

  createVideoCutJob: (payload: CreateVideoCutJobRequest) =>
    fetcher<CreateVideoCutJobResponse>("/v1/jobs/video-cut", {
      method: "POST",
      body: JSON.stringify({
        url: payload.url,
        format_id: payload.formatId,
        cut_mode: payload.cutMode,
        ...(payload.manual
          ? {
              manual: {
                start_sec: payload.manual.startSec,
                end_sec: payload.manual.endSec,
              },
            }
          : {}),
        ...(payload.heatmap
          ? {
              heatmap: {
                ...(typeof payload.heatmap.targetSec === "number"
                  ? { target_sec: payload.heatmap.targetSec }
                  : {}),
                ...(typeof payload.heatmap.windowSec === "number"
                  ? { window_sec: payload.heatmap.windowSec }
                  : {}),
              },
            }
          : {}),
      }),
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

  historyCreate: (payload: {
    url: string;
    platform: string;
    title: string;
    thumbnail_url: string;
  }) =>
    fetcher<{ ok: boolean }>("/v1/history", {
      method: "POST",
      body: JSON.stringify(payload),
    }),

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

  listAdminUsers: (limit = 20, offset = 0) =>
    fetcher<{
      items: AuthUser[];
      page: { total: number; limit: number; offset: number };
    }>(`/v1/admin/users?limit=${limit}&offset=${offset}`),

  getAdminUsersStats: () =>
    fetcher<AdminUsersStatsResponse>("/v1/admin/users/stats"),

  getAdminUser: (id: string) =>
    fetcher<AuthUser>(`/v1/admin/users/${encodeURIComponent(id)}`),

  updateAdminUser: (
    id: string,
    payload: {
      full_name?: string;
      role?: UserRole;
      plan?: UserPlan;
      plan_expires_at?: string;
    },
  ) =>
    fetcher<AuthUser>(`/v1/admin/users/${encodeURIComponent(id)}`, {
      method: "PATCH",
      body: JSON.stringify(payload),
    }),

  adminJobs: (limit = 30) =>
    fetcher<{ items: AdminJobRecord[] }>(`/admin/jobs?limit=${limit}`),
};
