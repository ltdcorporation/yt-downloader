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
import TikTokModal from "./TikTokModal";
import InstagramModal from "./InstagramModal";
import ProcessingModal from "./ProcessingModal";
import { api, type ResolveResponse } from "@/lib/api";
import { detectPlatform } from "@/lib/utils";

export default function InputBar() {
  const [url, setUrl] = useState("");
  const [resolvedUrl, setResolvedUrl] = useState("");
  const [resolveResult, setResolveResult] = useState<ResolveResponse | null>(null);
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isTikTokModalOpen, setIsTikTokModalOpen] = useState(false);
  const [isInstagramModalOpen, setIsInstagramModalOpen] = useState(false);
  const [isProcessingModalOpen, setIsProcessingModalOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const handleProcess = async (rawInput?: string) => {
    const targetUrl = (rawInput ?? url).trim();
    if (!targetUrl || isLoading) {
      return;
    }

    const platformType = detectPlatform(targetUrl);

    setErrorMessage("");
    setIsLoading(true);

    try {
      const result = await api.resolve(targetUrl);
      setResolvedUrl(targetUrl);
      setResolveResult(result);

      // Show platform-specific modal
      if (platformType === "tiktok") {
        setIsTikTokModalOpen(true);
      } else if (platformType === "instagram") {
        setIsInstagramModalOpen(true);
      } else {
        setIsModalOpen(true);
      }
    } catch (error) {
      setResolveResult(null);
      setIsModalOpen(false);
      setIsTikTokModalOpen(false);
      setIsInstagramModalOpen(false);
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

  const handleCloseTikTokModal = () => {
    setIsTikTokModalOpen(false);
  };

  const handleCloseInstagramModal = () => {
    setIsInstagramModalOpen(false);
  };

  const startFinalDownload = (formatId: string) => {
    if (!resolvedUrl || !resolveResult) return;

    // Close all potential source modals
    setIsModalOpen(false);
    setIsTikTokModalOpen(false);
    setIsInstagramModalOpen(false);

    // Show processing modal
    setIsProcessingModalOpen(true);

    // Mock delay for "processing" then trigger real download
    setTimeout(() => {
      const downloadUrl = api.getMp4DownloadUrl(resolvedUrl, formatId);
      window.location.href = downloadUrl;

      // Keep processing modal for a bit longer to show "Done" status
      setTimeout(() => {
        setIsProcessingModalOpen(false);
      }, 4000);
    }, 2500);
  };

  const handleInstagramDownload = () => {
    if (!resolvedUrl || !resolveResult) {
      return;
    }

    const mp4Formats = resolveResult.formats.filter(
      (format) => format.type === "mp4",
    );
    const highestQualityFormat = mp4Formats.length > 0
      ? mp4Formats[mp4Formats.length - 1]
      : null;

    if (highestQualityFormat) {
      startFinalDownload(highestQualityFormat.id);
    }
  };

  const handleTikTokDownloadNoWatermark = () => {
    if (!resolvedUrl || !resolveResult) {
      return;
    }

    const mp4Formats = resolveResult.formats.filter(
      (format) => format.type === "mp4",
    );
    const highestQualityFormat = mp4Formats.length > 0
      ? mp4Formats[mp4Formats.length - 1]
      : null;

    if (highestQualityFormat) {
      startFinalDownload(highestQualityFormat.id);
    }
  };

  const handleTikTokDownloadWithWatermark = () => {
    if (!resolvedUrl || !resolveResult) {
      return;
    }

    const mp4Formats = resolveResult.formats.filter(
      (format) => format.type === "mp4",
    );
    const lowestQualityFormat = mp4Formats.length > 0 ? mp4Formats[0] : null;

    if (lowestQualityFormat) {
      startFinalDownload(lowestQualityFormat.id);
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
            <div className="rounded-xl border border-rose-200 bg-rose-50 text-rose-700 px-4 py-3 text-sm flex items-start gap-2">
              <WarningCircle size={18} className="mt-0.5 flex-shrink-0" />
              <span>{errorMessage}</span>
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
      />

      <TikTokModal
        isOpen={isTikTokModalOpen}
        onClose={handleCloseTikTokModal}
        thumbnail={resolveResult?.thumbnail ?? null}
        title={resolveResult?.title ?? ""}
        author={resolveResult?.author ?? ""}
        views={resolveResult?.views ?? "0"}
        likes={resolveResult?.likes ?? "0"}
        shares={resolveResult?.shares ?? "0"}
        onDownloadNoWatermark={handleTikTokDownloadNoWatermark}
        onDownloadWithWatermark={handleTikTokDownloadWithWatermark}
      />

      <InstagramModal
        isOpen={isInstagramModalOpen}
        onClose={handleCloseInstagramModal}
        thumbnail={resolveResult?.thumbnail ?? null}
        title={resolveResult?.title ?? ""}
        author={resolveResult?.author ?? ""}
        views={resolveResult?.views ?? "0"}
        likes={resolveResult?.likes ?? "0"}
        shares={resolveResult?.shares ?? "0"}
        onDownload={handleInstagramDownload}
      />

      <ProcessingModal 
        isOpen={isProcessingModalOpen} 
        title={resolveResult?.title}
      />
    </>
  );
}
