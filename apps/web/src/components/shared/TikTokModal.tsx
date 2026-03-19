"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { X, Download, Drop, Play, Eye, Heart, ShareNetwork, TiktokLogo } from "@phosphor-icons/react";

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
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/80 backdrop-blur-sm p-4"
      style={{ width: "100vw", height: "100vh", top: 0, left: 0 }}
      onClick={onClose}
      role="dialog"
      aria-modal="true"
      aria-labelledby="tiktok-modal-title"
    >
      <div
        className="bg-white dark:bg-slate-900 w-full max-w-sm rounded-3xl shadow-2xl overflow-hidden border border-white/10 flex flex-col relative animate-in fade-in zoom-in duration-300"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Close button - Top right floating */}
        <button
          onClick={onClose}
          className="absolute top-4 right-4 z-10 bg-black/20 hover:bg-black/40 backdrop-blur-md text-white p-2 rounded-full transition-all border border-white/20"
          aria-label="Close modal"
        >
          <X size={20} weight="bold" />
        </button>

        {/* Video Preview Section */}
        <div className="relative w-full aspect-[9/16] bg-slate-900 overflow-hidden">
          {thumbnail ? (
            <div
              className="absolute inset-0 bg-center bg-no-repeat bg-cover"
              style={{ backgroundImage: `url("${thumbnail}")` }}
            >
              {/* Blur background for better effect */}
              <div className="absolute inset-0 backdrop-blur-3xl opacity-50 bg-black/40" />
              {/* Main image - contain to see full vertical video */}
              <div
                className="absolute inset-0 bg-center bg-no-repeat bg-contain"
                style={{ backgroundImage: `url("${thumbnail}")` }}
              />
            </div>
          ) : (
            <div className="absolute inset-0 flex items-center justify-center">
              <Play size={64} weight="fill" className="text-white/20" />
            </div>
          )}

          {/* TikTok Badge Overlay */}
          <div className="absolute top-4 left-4 flex items-center gap-2 bg-black/40 backdrop-blur-md px-3 py-1.5 rounded-full border border-white/20">
            <TiktokLogo size={18} weight="fill" className="text-white" />
            <span className="text-white text-xs font-bold tracking-tight">TikTok</span>
          </div>

          {/* Play Icon Overlay */}
          <div className="absolute inset-0 flex items-center justify-center pointer-events-none">
            <div className="bg-white/10 backdrop-blur-md rounded-full p-6 border border-white/20 transform scale-100 group-hover:scale-110 transition-transform">
              <Play size={40} weight="fill" className="text-white drop-shadow-lg" />
            </div>
          </div>

          {/* Video Title & Author Overlay (Bottom) */}
          <div className="absolute bottom-0 inset-x-0 p-6 bg-gradient-to-t from-black/90 via-black/40 to-transparent">
            <h1 className="text-white text-lg font-bold leading-tight line-clamp-2 drop-shadow-md">
              {title || "TikTok Video"}
            </h1>
            <p className="text-white/80 text-sm mt-1 font-medium drop-shadow-md">
              {author || "@tiktok_user"}
            </p>
          </div>
        </div>

        {/* Action Buttons Section */}
        <div className="p-6 bg-white dark:bg-slate-900">
          <div className="flex flex-col gap-3">
            <button
              onClick={onDownloadNoWatermark}
              className="flex items-center justify-center gap-3 bg-primary text-white py-4 px-6 rounded-2xl font-bold hover:brightness-110 transition-all shadow-lg shadow-primary/20 active:scale-[0.98]"
            >
              <Download size={24} weight="bold" />
              <span>Download No Watermark</span>
            </button>
            <button
              onClick={onDownloadWithWatermark}
              className="flex items-center justify-center gap-3 bg-slate-100 dark:bg-slate-800 text-slate-700 dark:text-slate-200 py-4 px-6 rounded-2xl font-bold hover:bg-slate-200 dark:hover:bg-slate-700 transition-all active:scale-[0.98]"
            >
              <Drop size={24} weight="bold" />
              <span>With Watermark</span>
            </button>
          </div>

          {/* Social Stats - Only show if data exists and is not "0" */}
          {(views && views !== "0") || (likes && likes !== "0") || (shares && shares !== "0") ? (
            <div className="mt-6 pt-6 border-t border-slate-100 dark:border-slate-800 flex justify-around items-center">
              {views && views !== "0" && (
                <div className="flex flex-col items-center gap-1">
                  <div className="text-slate-400 dark:text-slate-500">
                    <Eye size={20} weight="fill" />
                  </div>
                  <span className="text-xs font-bold text-slate-600 dark:text-slate-400">{views}</span>
                </div>
              )}
              {likes && likes !== "0" && (
                <div className="flex flex-col items-center gap-1">
                  <div className="text-rose-500">
                    <Heart size={20} weight="fill" />
                  </div>
                  <span className="text-xs font-bold text-slate-600 dark:text-slate-400">{likes}</span>
                </div>
              )}
              {shares && shares !== "0" && (
                <div className="flex flex-col items-center gap-1">
                  <div className="text-sky-500">
                    <ShareNetwork size={20} weight="fill" />
                  </div>
                  <span className="text-xs font-bold text-slate-600 dark:text-slate-400">{shares}</span>
                </div>
              )}
            </div>
          ) : (
            <p className="mt-6 text-center text-[11px] text-slate-400 italic">
              Ready to download in high quality
            </p>
          )}
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
}
