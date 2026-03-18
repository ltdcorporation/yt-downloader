export interface VideoFormat {
  format_id: string;
  ext: string;
  resolution: string;
  filesize?: number;
  url: string;
}

export interface VideoMetadata {
  id: string;
  title: string;
  thumbnail: string;
  duration: number;
  uploader: string;
  formats: VideoFormat[];
}

export interface DownloadJob {
  id: string;
  url: string;
  status: "pending" | "processing" | "done" | "failed";
  download_url?: string;
  error?: string;
  created_at: string;
  completed_at?: string;
}

export interface ApiError {
  message: string;
  code?: string;
}
