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

  getJobStatus: (jobId: string) => fetcher<JobStatusResponse>(`/v1/jobs/${jobId}`),

  getMp4DownloadUrl: (url: string, formatId: string) =>
    `${API_BASE_URL}/v1/download/mp4?url=${encodeURIComponent(url)}&format_id=${encodeURIComponent(formatId)}`,
};
