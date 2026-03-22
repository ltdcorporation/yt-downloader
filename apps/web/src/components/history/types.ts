export type HistoryPlatform = "youtube" | "tiktok" | "instagram" | "x";

export type HistoryTabPlatform = "all" | HistoryPlatform;

export type HistoryRequestKind = "mp3" | "mp4" | "image";

export type HistoryAttemptStatus =
  | "queued"
  | "processing"
  | "done"
  | "failed"
  | "expired";

export interface HistoryTableItem {
  id: string;
  title: string;
  quality: string;
  platform: HistoryPlatform;
  size: string;
  date: string;
  thumbnail: string;
  status: HistoryAttemptStatus;
  requestKind: HistoryRequestKind;
}

export interface HistoryStatCard {
  icon: "download" | "database" | "calendar";
  label: string;
  value: string;
}
