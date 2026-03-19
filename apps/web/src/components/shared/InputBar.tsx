"use client";

import { useState } from "react";
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
import { api, type ResolveResponse } from "@/lib/api";
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
  const [lastAttemptUrl, setLastAttemptUrl] = useState("");

  const troubleshootingItems = (() => {
    if (!errorMessage) return [];

    const message = errorMessage.toLowerCase();
    const items: string[] = [];

    items.push(
      "Pastikan link YouTube (watch / youtu.be / shorts), bukan playlist.",
    );
    items.push("Pastikan backend API berjalan di http://localhost:8080.");

    if (message.includes("only youtube")) {
      items.push(
        "Saat ini hanya YouTube yang didukung (TikTok/IG/X belum aktif di backend).",
      );
    }
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

    return items;
  })();

  const handleProcess = async (rawInput?: string) => {
    const targetUrl = (rawInput ?? url).trim();
    if (!targetUrl || isLoading) {
      return;
    }

    const platformType = detectPlatform(targetUrl);

    setErrorMessage("");
    setIsLoading(true);
    setLastAttemptUrl(targetUrl);

    try {
      const result = await api.resolve(targetUrl);
      setResolvedUrl(targetUrl);
      setResolveResult(result);

      // Show DownloadModal for all supported platforms
      setIsModalOpen(true);
    } catch (error) {
      setResolveResult(null);
      setIsModalOpen(false);
      setErrorMessage(
        error instanceof Error ? error.message : "Failed to resolve video URL.",
      );
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

    // Close all potential source modals
    setIsModalOpen(false);

    // Show processing modal
    setIsProcessingModalOpen(true);

    // Mock delay for "processing" then trigger real download
    setTimeout(() => {
      const downloadUrl = api.getMp4DownloadUrl(resolvedUrl, formatId);

      // Trigger download using an anchor element with 'download' attribute
      const link = document.createElement("a");
      link.href = downloadUrl;
      link.setAttribute("download", "");
      document.body.appendChild(link);
      link.click();
      document.body.removeChild(link);

      // Keep processing modal for a bit longer to show "Done" status
      setTimeout(() => {
        setIsProcessingModalOpen(false);
      }, 4000);
    }, 2500);
  };

  const startMp3Download = async () => {
    if (!resolvedUrl) return;

    setIsModalOpen(false);
    setIsProcessingModalOpen(true);

    try {
      await api.createMp3Job(resolvedUrl);
      // For MP3, we might want to keep the modal open to show "Done" status
      // In a real app, you'd poll for status, but here we just mock the "Done" state in ProcessingModal
    } catch (error) {
      setErrorMessage(
        error instanceof Error
          ? error.message
          : "Failed to start MP3 conversion.",
      );
      setIsProcessingModalOpen(false);
    }
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    const nextValue = e.target.value;
    setUrl(nextValue);
    setErrorMessage("");

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
      />
    </>
  );
}
