const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "/api";

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

export async function fetcher<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const errorPayload = await response
      .json()
      .catch(() => ({ error: "An error occurred" }));

    throw new Error(
      errorPayload?.error || errorPayload?.message || `HTTP ${response.status}`,
    );
  }

  return response.json();
}

export const api = {
  health: () => fetcher<{ ok: boolean; service: string; time: string }>("/healthz"),

  resolve: (url: string) =>
    fetcher<ResolveResponse>("/v1/youtube/resolve", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),

  createMp3Job: (url: string) =>
    fetcher<CreateMp3JobResponse>("/v1/jobs/mp3", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),

  getJobStatus: (jobId: string) => fetcher<JobStatusResponse>(`/v1/jobs/${jobId}`),

  getMp4DownloadUrl: (url: string, formatId: string) =>
    `${API_BASE_URL}/v1/download/mp4?url=${encodeURIComponent(url)}&format_id=${encodeURIComponent(formatId)}`,
};
