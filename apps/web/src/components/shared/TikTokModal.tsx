"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { X, Download, Drop, Play, Eye, Heart, ShareNetwork, Gear } from "@phosphor-icons/react";

interface TikTokModalProps {
  isOpen: boolean;
  onClose: () => void;
  thumbnail?: string | null;
  title?: string;
  author?: string;
  views?: string;
  likes?: string;
  shares?: string;
  onDownloadNoWatermark: () => void;
  onDownloadWithWatermark: () => void;
}

export default function TikTokModal({
  isOpen,
  onClose,
  thumbnail,
  title,
  author,
  views,
  likes,
  shares,
  onDownloadNoWatermark,
  onDownloadWithWatermark,
}: TikTokModalProps) {
  const [isMounted, setIsMounted] = useState(false);

  useEffect(() => {
    setIsMounted(true);
  }, []);

  useEffect(() => {
    if (isOpen) {
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }

    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  if (!isOpen || !isMounted) {
    return null;
  }

  const modalContent = (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/60 backdrop-blur-lg"
      style={{ width: "100vw", height: "100vh", top: 0, left: 0 }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="tiktok-modal-title"
    >
      <div
        className="bg-[#FFE2E2] dark:bg-slate-900 w-full max-w-sm rounded-xl shadow-lg overflow-hidden border border-primary/10"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header Section */}
        <div className="flex items-center justify-between px-6 py-4 border-b border-primary/10">
          <div className="flex items-center gap-2">
            <span className="text-primary">
              <Play size={24} weight="fill" />
            </span>
            <h2 id="tiktok-modal-title" className="text-primary font-bold text-lg">
              TikTok Downloader
            </h2>
          </div>
          <button
            className="text-primary hover:bg-primary/10 p-2 rounded-full transition-colors"
            aria-label="Settings"
          >
            <Gear size={20} weight="fill" />
          </button>
        </div>

        {/* Content Body */}
        <div className="p-6 flex flex-col items-center">
          {/* Vertical Video Thumbnail */}
          <div className="relative w-full aspect-[9/16] max-h-[320px] bg-slate-200 dark:bg-slate-700 rounded-lg overflow-hidden shadow-md group">
            {thumbnail ? (
              <div
                className="absolute inset-0 bg-center bg-no-repeat bg-cover"
                style={{ backgroundImage: `url("${thumbnail}")` }}
              />
            ) : (
              <div className="absolute inset-0 bg-slate-300 dark:bg-slate-600 flex items-center justify-center">
                <Play size={48} weight="fill" className="text-slate-400 dark:text-slate-500" />
              </div>
            )}
            <div className="absolute inset-0 bg-black/10 flex items-center justify-center">
              <div className="bg-white/20 backdrop-blur-md rounded-full p-4 border border-white/30">
                <Play size={40} weight="fill" className="text-white" />
              </div>
            </div>
          </div>

          {/* Video Info */}
          <div className="mt-6 w-full text-center">
            <h1 className="text-primary dark:text-slate-100 text-xl font-bold leading-tight line-clamp-2">
              {title || "Amazing City Sunset Vlog #Photography"}
            </h1>
            <p className="text-primary/70 dark:text-slate-400 text-base mt-1">
              {author || "@creator_vibe_official"}
            </p>
          </div>

          {/* Action Buttons */}
          <div className="mt-8 grid grid-cols-1 sm:grid-cols-2 gap-3 w-full">
            <button
              onClick={onDownloadNoWatermark}
              className="flex items-center justify-center gap-2 bg-primary text-white py-3 px-4 rounded-lg font-semibold hover:opacity-90 transition-opacity"
            >
              <Download size={20} weight="fill" />
              <span className="text-sm">No Watermark</span>
            </button>
            <button
              onClick={onDownloadWithWatermark}
              className="flex items-center justify-center gap-2 border-2 border-primary text-primary py-3 px-4 rounded-lg font-semibold hover:bg-primary/10 transition-colors"
            >
              <Drop size={20} weight="fill" />
              <span className="text-sm">With Watermark</span>
            </button>
          </div>
        </div>

        {/* Footer Meta */}
        <div className="px-6 py-4 bg-primary/5 dark:bg-primary/10 flex justify-center gap-4">
          <div className="flex items-center gap-1 text-xs text-primary/60 dark:text-slate-400">
            <Eye size={14} weight="fill" />
            <span>{views || "1.2M"}</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-primary/60 dark:text-slate-400">
            <Heart size={14} weight="fill" />
            <span>{likes || "45.2K"}</span>
          </div>
          <div className="flex items-center gap-1 text-xs text-primary/60 dark:text-slate-400">
            <ShareNetwork size={14} weight="fill" />
            <span>{shares || "12.8K"}</span>
          </div>
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
}
