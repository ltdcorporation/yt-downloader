const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

export async function fetcher<T>(endpoint: string, options?: RequestInit): Promise<T> {
  const response = await fetch(`${API_BASE_URL}${endpoint}`, {
    ...options,
    headers: {
      "Content-Type": "application/json",
      ...options?.headers,
    },
  });

  if (!response.ok) {
    const error = await response.json().catch(() => ({ message: "An error occurred" }));
    throw new Error(error.message || `HTTP ${response.status}`);
  }

  return response.json();
}

export const api = {
  // Health check
  health: () => fetcher<{ ok: boolean }>("/healthz"),
  
  // YouTube resolve
  resolve: (url: string) =>
    fetcher("/v1/youtube/resolve", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),
  
  // MP3 job
  createMp3Job: (url: string) =>
    fetcher<{ job_id: string }>("/v1/jobs/mp3", {
      method: "POST",
      body: JSON.stringify({ url }),
    }),
  
  getJobStatus: (jobId: string) =>
    fetcher<{ status: string; download_url?: string }>(`/v1/jobs/${jobId}`),
  
  // MP4 download
  getMp4DownloadUrl: (url: string, formatId: string) =>
    `/v1/download/mp4?url=${encodeURIComponent(url)}&format_id=${formatId}`,
};
