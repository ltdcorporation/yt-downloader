"use client";

import { useEffect, useMemo, useState } from "react";
import { createPortal } from "react-dom";
import { X, Download, YoutubeLogo, TiktokLogo, InstagramLogo, XLogo, WarningCircle } from "@phosphor-icons/react";
import { api, type ResolveFormat, type ResolveResponse } from "@/lib/api";
import { detectPlatform } from "@/lib/utils";

interface DownloadModalProps {
  isOpen: boolean;
  onClose: () => void;
  sourceUrl: string;
  result: ResolveResponse | null;
  isLoading?: boolean;
  onConfirmDownload: (formatId: string) => void;
}

function parseQualityToHeight(quality: string): number {
  const parsed = Number.parseInt(quality.replace(/[^0-9]/g, ""), 10);
  return Number.isFinite(parsed) ? parsed : 0;
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
    onConfirmDownload,
  }: DownloadModalProps) {
    const [selectedFormatId, setSelectedFormatId] = useState("");
    const [isConfirming, setIsConfirming] = useState(false);
  
    const platform = detectPlatform(sourceUrl);
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

    return [...formats].sort(
      (a, b) => parseQualityToHeight(a.quality) - parseQualityToHeight(b.quality),
    );
  }, [result]);

  const selectedFormat = useMemo(
    () => mp4Formats.find((format) => format.id === selectedFormatId) || null,
    [mp4Formats, selectedFormatId],
  );

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
      setIsConfirming(false);
    }

    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

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

      // default to highest available quality
      return mp4Formats[mp4Formats.length - 1]?.id || "";
    });
  }, [isOpen, mp4Formats]);

  if (!isOpen) {
    return null;
  }

  const handleDownloadTrigger = () => {
    if (!sourceUrl || !selectedFormat) {
      return;
    }
    setIsConfirming(true);
  };

  const handleFinalDownload = () => {
    if (!sourceUrl || !selectedFormat) {
      return;
    }

    onConfirmDownload(selectedFormat.id);
    setIsConfirming(false);
  };

  const hasFormats = mp4Formats.length > 0;

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
                You are about to download <strong>{result?.title}</strong> in <strong>{selectedFormat?.quality}</strong>.
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
              <p className="mt-4 text-[10px] text-slate-400 uppercase tracking-widest font-bold">
                Mock Confirmation v1
              </p>
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
              <div className="absolute bottom-1 right-1 bg-black/70 text-white text-[11px] px-1.5 py-0.5 rounded">
                {formatDuration(result?.duration_seconds)}
              </div>
            </div>

            <div className="min-w-0 flex-1 flex flex-col justify-center">
              <div className="flex items-center gap-2 mb-2">
                <span className={`${platformColor} text-[11px] font-bold px-2 py-0.5 rounded flex items-center gap-1`}>
                  <PlatformIcon size={12} weight="fill" />
                  {platformLabel}
                </span>
                {isLoading ? (
                  <span className="text-slate-400 text-xs">Resolving formats...</span>
                ) : null}
              </div>

              <h3 className="font-semibold text-slate-900 dark:text-slate-100 leading-tight line-clamp-2">
                {result?.title || "No metadata available"}
              </h3>

              {sourceUrl ? (
                <p className="text-xs text-slate-500 mt-2 truncate" title={sourceUrl}>
                  {sourceUrl}
                </p>
              ) : null}
            </div>
          </div>

          <div>
            <h4 className="text-sm font-bold text-slate-500 dark:text-slate-400 uppercase tracking-wider mb-3">
              Direct MP4 Download
            </h4>

            {hasFormats ? (
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                {mp4Formats.map((format: ResolveFormat) => {
                  const isSelected = format.id === selectedFormatId;
                  return (
                    <button
                      key={format.id}
                      onClick={() => setSelectedFormatId(format.id)}
                      className={`text-left p-4 border-2 rounded-xl transition-all ${
                        isSelected
                          ? "border-primary bg-primary/5"
                          : "border-slate-200 dark:border-slate-700 hover:border-primary/60"
                      }`}
                    >
                      <div className="flex items-center justify-between gap-2">
                        <span
                          className={`font-bold text-lg ${
                            isSelected
                              ? "text-primary"
                              : "text-slate-800 dark:text-slate-100"
                          }`}
                        >
                          {format.quality}
                        </span>
                        <span className="text-xs px-2 py-1 rounded bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 uppercase">
                          {format.container}
                        </span>
                      </div>

                      <p className="text-xs mt-2 text-slate-500 dark:text-slate-400">
                        {formatFileSize(format.filesize)}
                      </p>
                    </button>
                  );
                })}
              </div>
            ) : (
              <div className="rounded-xl border border-amber-200 bg-amber-50 text-amber-700 px-4 py-3 text-sm flex items-start gap-2">
                <WarningCircle size={18} className="mt-0.5 flex-shrink-0" />
                <span>
                  No MP4 download options found for this URL. Try another public {platformLabel} video.
                </span>
              </div>
            )}
          </div>

          <div className="pt-4 border-t border-slate-100 dark:border-slate-800">
            <button
              onClick={handleDownloadTrigger}
              disabled={!selectedFormat || !sourceUrl}
              className="w-full bg-primary text-white py-4 rounded-xl font-bold text-lg hover:brightness-105 transition-all shadow-lg shadow-primary/20 flex items-center justify-center gap-3 disabled:opacity-50 disabled:cursor-not-allowed"
            >
              <Download size={24} weight="bold" />
              {selectedFormat
                ? `Download MP4 (${selectedFormat.quality})`
                : "Download MP4"}
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
