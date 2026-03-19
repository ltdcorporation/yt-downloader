export function formatDuration(seconds: number): string {
  const hrs = Math.floor(seconds / 3600);
  const mins = Math.floor((seconds % 3600) / 60);
  const secs = seconds % 60;

  if (hrs > 0) {
    return `${hrs}:${mins.toString().padStart(2, "0")}:${secs.toString().padStart(2, "0")}`;
  }
  return `${mins}:${secs.toString().padStart(2, "0")}`;
}

export function formatFileSize(bytes: number): string {
  if (bytes === 0) return "0 B";
  const k = 1024;
  const sizes = ["B", "KB", "MB", "GB"];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return `${parseFloat((bytes / Math.pow(k, i)).toFixed(2))} ${sizes[i]}`;
}

export function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) return text;
  return `${text.slice(0, maxLength)}...`;
}

export type Platform = "youtube" | "tiktok" | "instagram" | "x" | "unknown";

export function detectPlatform(url: string): Platform {
  if (!url) return "unknown";
  
  try {
    const parsedUrl = new URL(url.trim());
    const hostname = parsedUrl.hostname.toLowerCase();

    if (hostname.includes("youtube.com") || hostname.includes("youtu.be")) {
      return "youtube";
    }
    if (hostname.includes("tiktok.com")) {
      return "tiktok";
    }
    if (hostname.includes("instagram.com")) {
      return "instagram";
    }
    if (hostname.includes("twitter.com") || hostname.includes("x.com")) {
      return "x";
    }
    
    return "unknown";
  } catch {
    // If URL parsing fails, fallback to simple string matching
    const lowerUrl = url.toLowerCase();
    if (lowerUrl.includes("youtube.com") || lowerUrl.includes("youtu.be")) {
      return "youtube";
    }
    if (lowerUrl.includes("tiktok.com")) {
      return "tiktok";
    }
    if (lowerUrl.includes("instagram.com")) {
      return "instagram";
    }
    if (lowerUrl.includes("twitter.com") || lowerUrl.includes("x.com")) {
      return "x";
    }
    
    return "unknown";
  }
}
