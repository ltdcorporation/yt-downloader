"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import {
  LinkSimpleHorizontal,
  Clipboard,
  DownloadSimple,
  WarningCircle,
  YoutubeLogo,
  TiktokLogo,
  InstagramLogo,
  XLogo,
} from "@phosphor-icons/react";
import DownloadModal from "./DownloadModal";
import ProcessingModal from "./ProcessingModal";
import { api, APIError, type ResolveResponse } from "@/lib/api";
import { detectPlatform } from "@/lib/utils";

export default function InputBar() {
  const [url, setUrl] = useState("");
  const [resolvedUrl, setResolvedUrl] = useState("");
  const [resolveResult, setResolveResult] = useState<ResolveResponse | null>(
    null,
  );
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isProcessingModalOpen, setIsProcessingModalOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");
  const [resolveErrorCode, setResolveErrorCode] = useState("");
  const [lastAttemptUrl, setLastAttemptUrl] = useState("");
  const [processingKind, setProcessingKind] = useState<"mp4" | "mp3" | null>(
    null,
  );
  const [mp3JobId, setMp3JobId] = useState("");
  const [mp3JobStatus, setMp3JobStatus] = useState("");
  const [mp3DownloadUrl, setMp3DownloadUrl] = useState("");
  const [mp3JobError, setMp3JobError] = useState("");

  const troubleshootingItems = (() => {
    if (!errorMessage) return [];

    const message = errorMessage.toLowerCase();
    const items: string[] = [];

    items.push("Pastikan link valid (YouTube / Instagram / TikTok / X).");
    items.push("Pastikan backend API berjalan di http://localhost:8080.");

    if (message.includes("rate limit")) {
      items.push("Tunggu beberapa detik lalu coba lagi (kena rate limit).");
    }
    if (message.includes("live")) {
      items.push("Video live/upcoming tidak didukung. Coba video biasa.");
    }
    if (message.includes("playlist")) {
      items.push("Playlist tidak didukung. Pakai link video tunggal.");
    }
    if (message.includes("failed to fetch") || message.includes("network")) {
      items.push(
        "Cek koneksi internet dan CORS (Origin localhost:3000 harus di-allow).",
      );
    }
    if (message.includes("hls-only") || message.includes("hls")) {
      items.push(
        "Konten HLS-only (m3u8) belum didukung untuk download MP4 langsung.",
      );
      items.push("Coba link lain yang punya direct MP4/progressive.");
    }

    return items;
  })();

  const closeProcessingModal = useCallback(() => {
    setIsProcessingModalOpen(false);
    setProcessingKind(null);
    setMp3JobId("");
    setMp3JobStatus("");
    setMp3DownloadUrl("");
    setMp3JobError("");
  }, []);

  const handleProcess = async (rawInput?: string) => {
    const targetUrl = (rawInput ?? url).trim();
    if (!targetUrl || isLoading) {
      return;
    }

    setLastAttemptUrl(targetUrl);
    setErrorMessage("");
    setResolveErrorCode("");

    setIsLoading(true);

    try {
      const result = await api.resolve(targetUrl);
      setResolvedUrl(targetUrl);
      setResolveResult(result);

      // Show DownloadModal for all supported platforms
      setIsModalOpen(true);
    } catch (error) {
      setResolveResult(null);
      setIsModalOpen(false);
      if (error instanceof APIError) {
        setResolveErrorCode(error.code || "");
        if (error.code === "x_hls_only_not_supported") {
          setErrorMessage(
            "Video X ini HLS-only (m3u8) dan belum didukung untuk download (by design).",
          );
        } else if (error.code === "ig_hls_only_not_supported") {
          setErrorMessage(
            "Konten Instagram ini HLS-only (m3u8) dan belum didukung untuk download (by design).",
          );
        } else if (error.code === "tt_hls_only_not_supported") {
          setErrorMessage(
            "Konten TikTok ini HLS-only (m3u8) dan belum didukung untuk download (by design).",
          );
        } else {
          setErrorMessage(error.message || "Failed to resolve video URL.");
        }
      } else {
        setResolveErrorCode("");
        setErrorMessage(
          error instanceof Error
            ? error.message
            : "Failed to resolve video URL.",
        );
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      setUrl(text);

      if (text.trim()) {
        await handleProcess(text);
      }
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? `Clipboard access failed: ${error.message}`
          : "Clipboard access failed.",
      );
    }
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
  };

  const startFinalDownload = (formatId: string) => {
    if (!resolvedUrl || !resolveResult) return;

    setIsModalOpen(false);
    setProcessingKind("mp4");
    setIsProcessingModalOpen(true);
    const downloadUrl = api.getMp4DownloadUrl(resolvedUrl, formatId);

    const link = document.createElement("a");
    link.href = downloadUrl;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);

    window.setTimeout(() => {
      closeProcessingModal();
    }, 1200);
  };

  const startMp3Download = useCallback(async () => {
    if (!resolvedUrl) return;

    setIsModalOpen(false);
    setProcessingKind("mp3");
    setIsProcessingModalOpen(true);
    setMp3JobId("");
    setMp3JobStatus("queued");
    setMp3DownloadUrl("");
    setMp3JobError("");

    try {
      const created = await api.createMp3Job(resolvedUrl);
      setMp3JobId(created.job_id);
      setMp3JobStatus(created.status || "queued");
    } catch (error) {
      setMp3JobStatus("failed");
      setMp3JobError(
        error instanceof Error
          ? error.message
          : "Failed to start MP3 conversion.",
      );
    }
  }, [resolvedUrl]);

  useEffect(() => {
    if (!isProcessingModalOpen || processingKind !== "mp3" || !mp3JobId) {
      return;
    }

    let cancelled = false;
    let timeoutId: ReturnType<typeof setTimeout> | null = null;

    const pollOnce = async () => {
      try {
        const status = await api.getJobStatus(mp3JobId);
        if (cancelled) return;

        setMp3JobStatus(status.status || "");
        setMp3DownloadUrl(status.download_url || "");
        setMp3JobError(status.error || "");

        if (status.status === "done" || status.status === "failed") {
          return;
        }

        timeoutId = setTimeout(pollOnce, 2000);
      } catch (error) {
        if (cancelled) return;
        setMp3JobError(
          error instanceof Error
            ? error.message
            : "Failed to check job status.",
        );
        timeoutId = setTimeout(pollOnce, 2500);
      }
    };

    void pollOnce();

    return () => {
      cancelled = true;
      if (timeoutId) {
        clearTimeout(timeoutId);
      }
    };
  }, [isProcessingModalOpen, processingKind, mp3JobId]);

  type ProcessingModalState = {
    status: "processing" | "completed" | "failed";
    heading: string;
    description: string;
    primaryActionLabel?: string;
    onPrimaryAction?: () => void;
    secondaryActionLabel?: string;
    onSecondaryAction?: () => void;
  };

  const processingModalState = useMemo<ProcessingModalState>(() => {
    if (!processingKind) {
      return {
        status: "processing",
        heading: "Memproses...",
        description: "Sedang menyiapkan file.",
      };
    }

    if (processingKind === "mp4") {
      return {
        status: "processing",
        heading: "Menyiapkan download",
        description: "Browser kamu akan mulai mengunduh setelah redirect siap.",
        secondaryActionLabel: "Tutup",
        onSecondaryAction: closeProcessingModal,
      };
    }

    const normalized = (mp3JobStatus || "").toLowerCase();
    const isDone = normalized === "done";
    const isFailed = normalized === "failed";
    const hasDownload = Boolean(mp3DownloadUrl);

    if (isDone && hasDownload) {
      return {
        status: "completed",
        heading: "MP3 siap",
        description: "Klik tombol di bawah untuk mengunduh file MP3.",
        primaryActionLabel: "Download MP3",
        onPrimaryAction: () => {
          const link = document.createElement("a");
          link.href = mp3DownloadUrl;
          link.target = "_blank";
          link.rel = "noopener noreferrer";
          document.body.appendChild(link);
          link.click();
          document.body.removeChild(link);
          closeProcessingModal();
        },
        secondaryActionLabel: "Tutup",
        onSecondaryAction: closeProcessingModal,
      };
    }

    if (isFailed) {
      return {
        status: "failed",
        heading: "Gagal membuat MP3",
        description: mp3JobError || "Job MP3 gagal diproses.",
        primaryActionLabel: "Coba lagi",
        onPrimaryAction: () => void startMp3Download(),
        secondaryActionLabel: "Tutup",
        onSecondaryAction: closeProcessingModal,
      };
    }

    return {
      status: "processing",
      heading:
        normalized === "processing" ? "Lagi convert MP3" : "Lagi antriin MP3",
      description: mp3JobId ? `Job ID: ${mp3JobId}` : "Sedang memulai job...",
      secondaryActionLabel: "Tutup",
      onSecondaryAction: closeProcessingModal,
    };
  }, [
    closeProcessingModal,
    mp3DownloadUrl,
    mp3JobError,
    mp3JobId,
    mp3JobStatus,
    processingKind,
    startMp3Download,
  ]);

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const nextValue = e.target.value;
    setUrl(nextValue);
    setErrorMessage("");
    setResolveErrorCode("");

    if (resolvedUrl && nextValue.trim() !== resolvedUrl.trim()) {
      setResolveResult(null);
    }
  };

  const handleInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      void handleProcess();
    }
  };

  const platform = detectPlatform(url);
  let PlatformIcon = LinkSimpleHorizontal;
  let placeholderText = "Paste your link here...";

  switch (platform) {
    case "youtube":
      PlatformIcon = YoutubeLogo;
      placeholderText = "Paste your YouTube URL here...";
      break;
    case "tiktok":
      PlatformIcon = TiktokLogo;
      placeholderText = "Paste your TikTok URL here...";
      break;
    case "instagram":
      PlatformIcon = InstagramLogo;
      placeholderText = "Paste your Instagram URL here...";
      break;
    case "x":
      PlatformIcon = XLogo;
      placeholderText = "Paste your X/Twitter URL here...";
      break;
  }

  return (
    <>
      <div className="relative w-full max-w-[640px] mx-auto group">
        <div className="flex flex-col gap-4">
          {/* Input Bar */}
          <div className="flex items-center bg-white dark:bg-slate-900 border border-primary/20 rounded-xl p-2 shadow-xl shadow-primary/5 focus-within:ring-2 focus-within:ring-primary/30 transition-all">
            <PlatformIcon
              size={20}
              weight="fill"
              className={`ml-3 flex-shrink-0 transition-colors ${
                platform !== "unknown" ? "text-primary" : "text-slate-400"
              }`}
            />
            <input
              className="flex-1 bg-transparent border-none focus:ring-0 text-slate-900 dark:text-slate-100 placeholder:text-slate-400 px-4 py-3 text-base md:text-lg"
              placeholder={placeholderText}
              type="text"
              value={url}
              onChange={handleInputChange}
              onKeyDown={handleInputKeyDown}
            />
            <button
              onClick={() => void handlePaste()}
              className="flex items-center gap-2 bg-primary text-white px-4 sm:px-6 py-3 rounded-lg font-bold hover:brightness-105 transition-all shadow-md flex-shrink-0"
            >
              <Clipboard size={20} weight="fill" />
              <span className="hidden sm:inline">Paste</span>
            </button>
          </div>

          {/* Process Button - Stacked below input on mobile */}
          <button
            onClick={() => void handleProcess()}
            disabled={!url.trim() || isLoading}
            className="w-full bg-primary text-white py-4 rounded-xl font-bold text-lg hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 flex items-center justify-center gap-3 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? (
              <>
                <span className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Resolving...</span>
              </>
            ) : (
              <>
                <DownloadSimple size={24} weight="fill" />
                <span>Resolve Download Options</span>
              </>
            )}
          </button>

          {platform === "x" && url.trim() ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-900">
              <div className="flex items-start gap-2">
                <WarningCircle
                  size={18}
                  className="mt-0.5 flex-shrink-0 text-amber-700"
                />
                <div className="min-w-0 flex-1">
                  <p className="font-bold">X / Twitter: bisa HLS-only</p>
                  <p className="mt-1 leading-relaxed">
                    Banyak video X bersifat HLS-only (m3u8). Kalau muncul
                    warning HLS-only, memang belum didukung (by design).
                  </p>
                </div>
              </div>
            </div>
          ) : null}

          {platform === "instagram" &&
          resolveErrorCode === "ig_hls_only_not_supported" ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-900">
              <div className="flex items-start gap-2">
                <WarningCircle
                  size={18}
                  className="mt-0.5 flex-shrink-0 text-amber-700"
                />
                <div className="min-w-0 flex-1">
                  <p className="font-bold">
                    Instagram: HLS-only (belum didukung)
                  </p>
                  <p className="mt-1 leading-relaxed">
                    Konten ini hanya tersedia sebagai HLS (m3u8), jadi belum
                    bisa diunduh via flow MP4 (by design).
                  </p>
                </div>
              </div>
            </div>
          ) : null}

          {platform === "tiktok" &&
          resolveErrorCode === "tt_hls_only_not_supported" ? (
            <div className="rounded-2xl border border-amber-200 bg-amber-50 px-4 py-4 text-sm text-amber-900">
              <div className="flex items-start gap-2">
                <WarningCircle
                  size={18}
                  className="mt-0.5 flex-shrink-0 text-amber-700"
                />
                <div className="min-w-0 flex-1">
                  <p className="font-bold">TikTok: HLS-only (belum didukung)</p>
                  <p className="mt-1 leading-relaxed">
                    Konten ini hanya tersedia sebagai HLS (m3u8), jadi belum
                    bisa diunduh via flow MP4 (by design).
                  </p>
                </div>
              </div>
            </div>
          ) : null}

          {errorMessage ? (
            <div className="rounded-2xl border border-rose-200 bg-rose-50 px-4 py-4 text-sm text-rose-800">
              <div className="flex items-start gap-2">
                <WarningCircle
                  size={18}
                  className="mt-0.5 flex-shrink-0 text-rose-700"
                />
                <div className="min-w-0 flex-1">
                  <p className="font-bold">Gagal memproses link</p>
                  <p className="mt-1 break-words">{errorMessage}</p>
                </div>
              </div>

              <div className="mt-4 flex flex-col sm:flex-row gap-2">
                <button
                  type="button"
                  onClick={() => void handleProcess(lastAttemptUrl || url)}
                  disabled={isLoading || !(lastAttemptUrl || url).trim()}
                  className="inline-flex items-center justify-center rounded-xl bg-primary px-4 py-2.5 font-bold text-white shadow-lg shadow-primary/20 hover:brightness-105 transition-all disabled:opacity-50 disabled:cursor-not-allowed"
                >
                  Coba lagi
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setErrorMessage("");
                    setResolveErrorCode("");
                  }}
                  className="inline-flex items-center justify-center rounded-xl bg-white/70 px-4 py-2.5 font-bold text-rose-700 border border-rose-200 hover:bg-white transition-all"
                >
                  Tutup
                </button>
              </div>

              {troubleshootingItems.length > 0 ? (
                <div className="mt-4 rounded-xl border border-rose-200/70 bg-white/60 px-4 py-3">
                  <p className="text-xs font-bold uppercase tracking-wider text-rose-700">
                    Troubleshooting
                  </p>
                  <ul className="mt-2 list-disc pl-5 text-rose-800/90 space-y-1">
                    {troubleshootingItems.map((item) => (
                      <li key={item}>{item}</li>
                    ))}
                  </ul>
                </div>
              ) : null}
            </div>
          ) : null}
        </div>
      </div>

      <DownloadModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        sourceUrl={resolvedUrl}
        result={resolveResult}
        isLoading={isLoading}
        onConfirmDownload={startFinalDownload}
        onConfirmMp3={startMp3Download}
        onRetryResolve={() => void handleProcess(resolvedUrl || url)}
      />

      <ProcessingModal
        isOpen={isProcessingModalOpen}
        title={resolveResult?.title}
        status={processingModalState.status}
        heading={processingModalState.heading}
        description={processingModalState.description}
        primaryActionLabel={processingModalState.primaryActionLabel}
        onPrimaryAction={processingModalState.onPrimaryAction}
        secondaryActionLabel={processingModalState.secondaryActionLabel}
        onSecondaryAction={processingModalState.onSecondaryAction}
        onClose={closeProcessingModal}
      />
    </>
  );
}
