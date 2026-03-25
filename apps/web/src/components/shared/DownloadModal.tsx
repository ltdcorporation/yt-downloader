"use client";

import { useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import {
  X,
  Download,
  YoutubeLogo,
  TiktokLogo,
  InstagramLogo,
  XLogo,
  WarningCircle,
  MusicNotes,
  Image as ImageIcon,
  CheckCircle,
  Scissors,
  Flame,
  Clock,
} from "@phosphor-icons/react";
import {
  type ResolveFormat,
  type ResolveResponse,
  type SettingsQuality,
} from "@/lib/api";
import { detectPlatform } from "@/lib/utils";

interface DownloadModalProps {
  isOpen: boolean;
  onClose: () => void;
  sourceUrl: string;
  result: ResolveResponse | null;
  isLoading?: boolean;
  preferredQuality?: SettingsQuality | null;
  onConfirmDownload: (formatId: string) => void;
  onConfirmMp3: () => void;
  onRetryResolve?: () => void;
}

function parseQualityToHeight(quality: string): number {
  const parsed = Number.parseInt(quality.replace(/[^0-9]/g, ""), 10);
  return Number.isFinite(parsed) ? parsed : 0;
}

function preferredQualityToHeight(
  preferredQuality?: SettingsQuality | null,
): number {
  switch (preferredQuality) {
    case "4k":
      return 2160;
    case "1080p":
      return 1080;
    case "720p":
      return 720;
    case "480p":
      return 480;
    default:
      return 0;
  }
}

function pickDefaultFormat(
  formats: ResolveFormat[],
  preferredQuality?: SettingsQuality | null,
): ResolveFormat | null {
  if (formats.length === 0) {
    return null;
  }

  const targetHeight = preferredQualityToHeight(preferredQuality);
  if (targetHeight <= 0) {
    return formats[formats.length - 1] ?? null;
  }

  let bestAtOrBelowTarget: ResolveFormat | null = null;
  for (const format of formats) {
    const height = parseQualityToHeight(format.quality);
    if (height <= targetHeight) {
      bestAtOrBelowTarget = format;
    }
  }

  return bestAtOrBelowTarget ?? formats[formats.length - 1] ?? null;
}

function formatDuration(totalSeconds?: number): string {
  if (!totalSeconds || totalSeconds <= 0) {
    return "--:--";
  }

  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;

  if (hours > 0) {
    return `${hours}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
  }

  return `${minutes}:${String(seconds).padStart(2, "0")}`;
}

function formatDurationToHMS(totalSeconds: number): string {
  const hours = Math.floor(totalSeconds / 3600);
  const minutes = Math.floor((totalSeconds % 3600) / 60);
  const seconds = totalSeconds % 60;
  return `${String(hours).padStart(2, "0")}:${String(minutes).padStart(2, "0")}:${String(seconds).padStart(2, "0")}`;
}

function formatFileSize(bytes?: number): string {
  if (!bytes || bytes <= 0) {
    return "Unknown size";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let value = bytes;
  let unitIndex = 0;

  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }

  const precision = value >= 100 ? 0 : value >= 10 ? 1 : 2;
  return `${value.toFixed(precision)} ${units[unitIndex]}`;
}

export default function DownloadModal({
  isOpen,
  onClose,
  sourceUrl,
  result,
  isLoading,
  preferredQuality,
  onConfirmDownload,
  onConfirmMp3,
  onRetryResolve,
}: DownloadModalProps) {
  const [selectedFormatId, setSelectedFormatId] = useState("");
  const [selectedMediaIds, setSelectedMediaIds] = useState<string[]>([]);
  const [isConfirming, setIsConfirming] = useState(false);

  // Time trim state
  const [startTime, setStartTime] = useState("00:00:00");
  const [endTime, setEndTime] = useState("00:00:00");
  const [isHeatmapCut, setIsHeatmapCut] = useState(false);

  const platform = detectPlatform(sourceUrl);
  const isYoutube = platform === "youtube";
  const isInstagram = platform === "instagram";
  const isCarousel = isInstagram && result?.kind === "carousel";
  const isImageOnly = isInstagram && result?.kind === "image";

  let PlatformIcon = YoutubeLogo;
  let platformLabel = "YouTube";
  let platformColor = "bg-red-100 text-red-600";

  switch (platform) {
    case "youtube":
      PlatformIcon = YoutubeLogo;
      platformLabel = "YouTube";
      platformColor = "bg-red-100 text-red-600";
      break;
    case "tiktok":
      PlatformIcon = TiktokLogo;
      platformLabel = "TikTok";
      platformColor = "bg-slate-900 text-white";
      break;
    case "instagram":
      PlatformIcon = InstagramLogo;
      platformLabel = "Instagram";
      platformColor = "bg-pink-100 text-pink-600";
      break;
    case "x":
      PlatformIcon = XLogo;
      platformLabel = "X / Twitter";
      platformColor = "bg-slate-900 text-white";
      break;
  }

  const mp4Formats = useMemo(() => {
    const formats = (result?.formats || []).filter(
      (format) => format.type === "mp4",
    );

    // Sort: SD (usually lower height) first, then HD
    return [...formats].sort(
      (a, b) =>
        parseQualityToHeight(a.quality) - parseQualityToHeight(b.quality),
    );
  }, [result]);

  const selectedFormat = useMemo(
    () => mp4Formats.find((format) => format.id === selectedFormatId) || null,
    [mp4Formats, selectedFormatId],
  );

  const toggleMediaSelection = (id: string) => {
    setSelectedMediaIds((prev) =>
      prev.includes(id) ? prev.filter((i) => i !== id) : [...prev, id],
    );
  };

  const selectAllMedias = () => {
    if (!result?.medias) return;
    if (selectedMediaIds.length === result.medias.length) {
      setSelectedMediaIds([]);
    } else {
      setSelectedMediaIds(result.medias.map((m) => m.id));
    }
  };

  const instagramHasVideoOnly = useMemo(() => {
    if (platform !== "instagram") return false;
    return mp4Formats.some((format) =>
      String(format.id).toLowerCase().startsWith("dash-"),
    );
  }, [mp4Formats, platform]);

  const handleTimeTrimChange = (type: "start" | "end", value: string) => {
    if (type === "start") setStartTime(value);
    else setEndTime(value);
    setIsHeatmapCut(false);
  };

  const handleHeatmapToggle = () => {
    const nextVal = !isHeatmapCut;
    setIsHeatmapCut(nextVal);
    if (nextVal && result?.duration_seconds) {
      setStartTime("00:00:00");
      setEndTime(formatDurationToHMS(result.duration_seconds));
    }
  };

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
      // Auto-select all if carousel
      if (result?.medias && result.kind === "carousel") {
        setSelectedMediaIds(result.medias.map((m) => m.id));
      }

      // Init time trim
      if (result?.duration_seconds) {
        setEndTime(formatDurationToHMS(result.duration_seconds));
        setStartTime("00:00:00");
        setIsHeatmapCut(false);
      }
    } else {
      document.body.style.overflow = "";
      setIsConfirming(false);
      setSelectedMediaIds([]);
    }

    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen, result]);

  useEffect(() => {
    if (!isOpen) {
      setSelectedFormatId("");
      return;
    }

    if (mp4Formats.length === 0) {
      setSelectedFormatId("");
      return;
    }

    setSelectedFormatId((prev) => {
      if (prev && mp4Formats.some((format) => format.id === prev)) {
        return prev;
      }

      const preferred = pickDefaultFormat(mp4Formats, preferredQuality);
      return preferred?.id || "";
    });
  }, [isOpen, mp4Formats, preferredQuality]);

  if (!isOpen) {
    return null;
  }

  const handleDownloadTrigger = () => {
    if (isCarousel) {
      if (selectedMediaIds.length === 0) return;
      setIsConfirming(true);
      return;
    }
    if (isImageOnly) {
      setIsConfirming(true);
      return;
    }
    if (!sourceUrl || !selectedFormat) {
      return;
    }
    setIsConfirming(true);
  };

  const handleFinalDownload = () => {
    if (isCarousel) {
      selectedMediaIds.forEach((id, index) => {
        setTimeout(() => {
          onConfirmDownload(id);
        }, index * 800);
      });
      setIsConfirming(false);
      return;
    }

    if (isImageOnly && result?.medias && result.medias.length > 0) {
      onConfirmDownload(result.medias[0].id);
      setIsConfirming(false);
      return;
    }

    if (!sourceUrl || !selectedFormat) {
      return;
    }

    onConfirmDownload(selectedFormat.id);
    setIsConfirming(false);
  };

  const hasFormats = mp4Formats.length > 0;

  const downloadButtonLabel = (() => {
    if (isCarousel) {
      return `Download ${selectedMediaIds.length} Selected Media`;
    }
    if (isImageOnly) {
      return "Download Image";
    }
    return selectedFormat
      ? `Download MP4 (${selectedFormat.quality})`
      : "Download MP4";
  })();

  const modalContent = (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/60 backdrop-blur-lg"
      style={{ width: "100vw", height: "100vh", top: 0, left: 0 }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="modal-title"
    >
      <div
        className="bg-white dark:bg-slate-950 w-full max-w-2xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[85vh] m-4 relative"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Mock Confirmation Modal Overlay */}
        {isConfirming && (
          <div className="absolute inset-0 z-[10000] bg-white/90 dark:bg-slate-950/90 backdrop-blur-sm flex items-center justify-center p-6 animate-in fade-in duration-200">
            <div className="bg-white dark:bg-slate-900 p-8 rounded-3xl shadow-2xl border border-slate-100 dark:border-slate-800 max-w-sm w-full text-center">
              <div className="w-16 h-16 bg-primary/10 rounded-full flex items-center justify-center mx-auto mb-4">
                <Download size={32} weight="bold" className="text-primary" />
              </div>
              <h3 className="text-xl font-bold text-slate-900 dark:text-slate-100 mb-2">
                Ready to Download?
              </h3>
              <p className="text-slate-500 dark:text-slate-400 text-sm mb-8 leading-relaxed">
                {isCarousel
                  ? `You are about to download ${selectedMediaIds.length} media items from this post.`
                  : isImageOnly
                    ? `You are about to download the photo from this post.`
                    : `You are about to download ${result?.title} in ${selectedFormat?.quality}.`}
              </p>
              <div className="flex flex-col gap-3">
                <button
                  onClick={handleFinalDownload}
                  className="w-full bg-primary text-white py-4 rounded-xl font-bold hover:brightness-105 transition-all shadow-lg shadow-primary/20"
                >
                  Confirm & Start Download
                </button>
                <button
                  onClick={() => setIsConfirming(false)}
                  className="w-full bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 py-4 rounded-xl font-bold hover:bg-slate-200 dark:hover:bg-slate-700 transition-all"
                >
                  Cancel
                </button>
              </div>
            </div>
          </div>
        )}
        <div className="px-6 md:px-8 pt-6 pb-4 flex justify-between items-center border-b border-slate-100 dark:border-slate-800">
          <h2 id="modal-title" className="text-xl md:text-2xl font-bold text-primary">
            Download Options
          </h2>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
            aria-label="Close modal"
          >
            <X size={24} />
          </button>
        </div>

        <div className="px-6 md:px-8 pb-8 pt-6 space-y-6 overflow-y-auto">
          <div className="flex gap-4 p-4 bg-slate-50 dark:bg-slate-900 rounded-xl border border-slate-100 dark:border-slate-800">
            <div className="relative w-36 h-24 bg-slate-200 dark:bg-slate-700 rounded-lg overflow-hidden flex-shrink-0">
              {result?.thumbnail ? (
                // eslint-disable-next-line @next/next/no-img-element
                <img
                  alt={result?.title || "Video thumbnail"}
                  className="w-full h-full object-cover"
                  src={result.thumbnail}
                />
              ) : null}
              {!isImageOnly && !isCarousel && (
                <div className="absolute bottom-1 right-1 bg-black/70 text-white text-[11px] px-1.5 py-0.5 rounded">
                  {formatDuration(result?.duration_seconds)}
                </div>
              )}
            </div>

            <div className="min-w-0 flex-1 flex flex-col justify-center">
              <div className="flex items-center gap-2 mb-2">
                <span className={`${platformColor} text-[11px] font-bold px-2 py-0.5 rounded flex items-center gap-1`}>
                  <PlatformIcon size={12} weight="fill" />
                  {platformLabel}
                </span>
                {isLoading && <span className="text-slate-400 text-xs">Resolving...</span>}
              </div>
              <h3 className="font-semibold text-slate-900 dark:text-slate-100 leading-tight line-clamp-2">
                {result?.title || "No metadata available"}
              </h3>
              {sourceUrl && (
                <p className="text-xs text-slate-500 mt-2 truncate" title={sourceUrl}>
                  {sourceUrl}
                </p>
              )}
            </div>
          </div>

          <div>
            <h4 className="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
              {isCarousel
                ? "Instagram Carousel (Pilih Media)"
                : isImageOnly
                  ? "Instagram Photo"
                  : "Direct MP4 Download"}
            </h4>

            {isInstagram && !isCarousel && !isImageOnly && (
              <div className="mb-3 rounded-2xl border border-slate-200 dark:border-slate-800 bg-white/60 dark:bg-slate-900/40 px-4 py-3 text-sm text-slate-700 dark:text-slate-200">
                <p className="font-bold">Instagram Reels: Pilih Kualitas</p>
                <p className="mt-1 text-slate-500 dark:text-slate-400 leading-relaxed">
                  Pilih antara SD (kualitas standar) atau HD (kualitas tinggi) jika tersedia.
                </p>
              </div>
            )}

            {isCarousel && result?.medias && (
              <div className="space-y-4">
                <div className="flex justify-between items-center">
                  <p className="text-xs text-slate-500">
                    {selectedMediaIds.length} dari {result.medias.length} media dipilih
                  </p>
                  <button onClick={selectAllMedias} className="text-xs font-bold text-primary hover:underline">
                    {selectedMediaIds.length === result.medias.length ? "Unselect All" : "Select All"}
                  </button>
                </div>
                <div className="grid grid-cols-2 sm:grid-cols-3 gap-3">
                  {result.medias.map((media) => {
                    const isSelected = selectedMediaIds.includes(media.id);
                    return (
                      <button
                        key={media.id}
                        onClick={() => toggleMediaSelection(media.id)}
                        className={`relative aspect-square rounded-xl overflow-hidden border-2 transition-all ${
                          isSelected ? "border-primary shadow-lg" : "border-slate-200 dark:border-slate-800 opacity-60"
                        }`}
                      >
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img src={media.thumbnail || media.url} alt="Instagram media" className="w-full h-full object-cover" />
                        <div className="absolute top-2 right-2">
                          {isSelected ? (
                            <CheckCircle size={20} weight="fill" className="text-primary bg-white rounded-full" />
                          ) : (
                            <div className="w-5 h-5 border-2 border-white rounded-full bg-black/20" />
                          )}
                        </div>
                        <div className="absolute bottom-2 left-2 bg-black/60 text-white text-[10px] px-1.5 py-0.5 rounded font-bold uppercase">
                          {media.type}
                        </div>
                      </button>
                    );
                  })}
                </div>
              </div>
            )}

            {isImageOnly && result?.medias && (
              <div className="grid grid-cols-1 gap-3">
                {result.medias.map((media) => (
                  <button
                    key={media.id}
                    onClick={() => onConfirmDownload(media.id)}
                    className="w-full flex items-center gap-4 p-4 border-2 border-slate-200 dark:border-slate-700 rounded-xl hover:border-primary/60 transition-all group"
                  >
                    <div className="w-12 h-12 bg-primary/10 rounded-lg flex items-center justify-center text-primary group-hover:bg-primary group-hover:text-white transition-colors">
                      <ImageIcon size={24} weight="bold" />
                    </div>
                    <div className="text-left flex-1">
                      <p className="font-bold text-slate-800 dark:text-slate-100">Download Image</p>
                      <p className="text-xs text-slate-500">High resolution JPG format</p>
                    </div>
                    <Download size={20} className="text-slate-400 group-hover:text-primary transition-colors" />
                  </button>
                ))}
              </div>
            )}

            {!isCarousel && !isImageOnly && hasFormats ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {mp4Formats.map((format: ResolveFormat) => {
                  const isSelected = format.id === selectedFormatId;
                  const isInstagramVideoOnly =
                    platform === "instagram" && String(format.id).toLowerCase().startsWith("dash-");
                  let label = format.quality;
                  if (isInstagram) {
                    const h = parseQualityToHeight(format.quality);
                    label = h >= 720 ? `HD (${format.quality})` : `SD (${format.quality})`;
                  }
                  return (
                    <button
                      key={format.id}
                      onClick={() => setSelectedFormatId(format.id)}
                      className={`text-left p-4 border-2 rounded-xl transition-all ${
                        isSelected ? "border-primary bg-primary/5" : "border-slate-200 dark:border-slate-700 hover:border-primary/60"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span className={`font-bold text-lg ${isSelected ? "text-primary" : "text-slate-800 dark:text-slate-100"}`}>
                          {label}
                        </span>
                        <div className="flex items-center gap-2">
                          {isInstagramVideoOnly && (
                            <span className="text-[10px] px-2 py-1 rounded bg-amber-100 text-amber-900 uppercase font-bold">
                              Video-only
                            </span>
                          )}
                          <span className="text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 uppercase">
                            {format.container}
                          </span>
                        </div>
                      </div>
                      <p className="text-xs mt-2 text-slate-500 dark:text-slate-400">{formatFileSize(format.filesize)}</p>
                    </button>
                  );
                })}
              </div>
            ) : null}

            {!isCarousel && !isImageOnly && !hasFormats && (
              <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-800">
                <div className="flex items-start gap-2">
                  <WarningCircle size={18} className="mt-0.5 flex-shrink-0 text-amber-700" />
                  <div className="min-w-0 flex-1">
                    <p className="font-bold">Tidak menemukan opsi download</p>
                    <p className="mt-1">Coba gunakan link lain atau resolve ulang.</p>
                  </div>
                </div>
              </div>
            )}

            {isInstagram && instagramHasVideoOnly && (
              <p className="mt-3 text-[11px] text-amber-700 dark:text-amber-300 leading-relaxed">
                Catatan: label Video-only artinya file bisa tanpa audio. Ini perilaku dari sumber Instagram (by design).
              </p>
            )}
          </div>

          {!isCarousel && !isImageOnly && isYoutube && (
            <div className="p-5 bg-slate-50 dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 space-y-4">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2 text-slate-900 dark:text-white font-bold text-sm">
                  <Scissors size={18} weight="fill" className="text-primary" />
                  Cutoff Video
                </div>
                <button
                  onClick={handleHeatmapToggle}
                  className={`flex items-center gap-2 px-3 py-1.5 rounded-lg text-[10px] font-black uppercase tracking-wider transition-all border ${
                    isHeatmapCut
                      ? "bg-orange-500 border-orange-400 text-white shadow-lg shadow-orange-500/20"
                      : "bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 text-slate-500"
                  }`}
                >
                  <Flame size={14} weight={isHeatmapCut ? "fill" : "bold"} />
                  Heatmap Cut {isHeatmapCut ? "ON" : "OFF"}
                </button>
              </div>

              <div className="grid grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <label className="text-[10px] font-black text-slate-400 uppercase tracking-widest ml-1">
                    Start Time
                  </label>
                  <div className="relative">
                    <Clock size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
                    <input
                      type="text"
                      value={startTime}
                      onChange={(e) => handleTimeTrimChange("start", e.target.value)}
                      disabled={isHeatmapCut}
                      className={`w-full pl-9 pr-3 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-xs font-bold focus:ring-2 focus:ring-primary/50 outline-none transition-all ${
                        isHeatmapCut ? "opacity-50 cursor-not-allowed bg-slate-50 dark:bg-slate-900" : ""
                      }`}
                      placeholder="00:00:00"
                    />
                  </div>
                </div>
                <div className="space-y-1.5">
                  <label className="text-[10px] font-black text-slate-400 uppercase tracking-widest ml-1">
                    End Time
                  </label>
                  <div className="relative">
                    <Clock size={14} className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400" />
                    <input
                      type="text"
                      value={endTime}
                      onChange={(e) => handleTimeTrimChange("end", e.target.value)}
                      disabled={isHeatmapCut}
                      className={`w-full pl-9 pr-3 py-2 bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg text-xs font-bold focus:ring-2 focus:ring-primary/50 outline-none transition-all ${
                        isHeatmapCut ? "opacity-50 cursor-not-allowed bg-slate-50 dark:bg-slate-900" : ""
                      }`}
                      placeholder="00:00:00"
                    />
                  </div>
                </div>
              </div>
              
              {isHeatmapCut && (
                <p className="text-[10px] text-orange-600 dark:text-orange-400 font-medium bg-orange-50 dark:bg-orange-950/30 p-2 rounded-lg border border-orange-100 dark:border-orange-900/50">
                  Note: Heatmap Cut will automatically find and cut the most viral/highlighted part of this video.
                </p>
              )}
            </div>
          )}

          {!isCarousel && !isImageOnly && (
            <div className="space-y-3">
              <h4 className="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider">Audio (MP3)</h4>
              <button
                onClick={onConfirmMp3}
                disabled={!sourceUrl || isLoading || platform !== "youtube"}
                className="w-full flex items-center justify-between p-4 border-2 border-slate-200 dark:border-slate-700 rounded-xl hover:border-primary/60 transition-all group"
              >
                <div className="flex items-center gap-3">
                  <div className="w-10 h-10 bg-primary/10 rounded-lg flex items-center justify-center text-primary group-hover:bg-primary group-hover:text-white transition-colors">
                    <MusicNotes size={20} weight="bold" />
                  </div>
                  <div className="text-left">
                    <p className="font-bold text-slate-800 dark:text-slate-100">MP3 Audio</p>
                    <p className="text-xs text-slate-500">
                      {platform === "youtube" ? "128kbps • Queue processing" : "Hanya tersedia untuk YouTube"}
                    </p>
                  </div>
                </div>
                <Download size={20} className="text-slate-400 group-hover:text-primary transition-colors" />
              </button>
            </div>
          )}

          <div className="pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              onClick={handleDownloadTrigger}
              disabled={
                (isCarousel && selectedMediaIds.length === 0) ||
                (!isCarousel && !isImageOnly && !selectedFormat) ||
                !sourceUrl
              }
              className={`w-full py-4 rounded-xl font-bold text-lg transition-all flex items-center justify-center gap-3 shadow-lg ${
                ((isCarousel && selectedMediaIds.length > 0) ||
                (isImageOnly) ||
                (!isCarousel && !isImageOnly && selectedFormat)) && sourceUrl
                  ? "bg-emerald-600 hover:bg-emerald-500 text-white shadow-emerald-500/20 active:scale-[0.98]"
                  : "bg-slate-200 dark:bg-slate-800 text-slate-400 dark:text-slate-600 cursor-not-allowed shadow-none"
              }`}
            >
              <Download size={24} weight="bold" />
              {downloadButtonLabel}
            </button>
            <p className="text-center text-[11px] text-slate-400 mt-4 leading-relaxed px-4 md:px-12">
              Downloads directly from source. No new tabs will be opened.
            </p>
          </div>
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
}
