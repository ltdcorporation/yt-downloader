"use client";

import { useEffect } from "react";
import { createPortal } from "react-dom";
import { X, Download, Scissors, YoutubeLogo } from "@phosphor-icons/react";

interface DownloadModalProps {
  isOpen: boolean;
  onClose: () => void;
  videoUrl?: string;
  isLoading?: boolean;
}

interface ResolutionOption {
  quality: string;
  format: string;
  size: string;
  recommended?: boolean;
}

const RESOLUTION_OPTIONS: ResolutionOption[] = [
  { quality: "720p", format: "MP4", size: "42MB", recommended: true },
  { quality: "1080p", format: "MP4", size: "128MB" },
  { quality: "4K", format: "MP4", size: "840MB" },
];

export default function DownloadModal({
  isOpen,
  onClose,
  videoUrl,
}: DownloadModalProps) {
  // Prevent body scroll when modal is open
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

  if (!isOpen) return null;

  const handleDownload = () => {
    console.log("Downloading:", videoUrl);
    // TODO: Implement download logic
  };

  const handleTrimToggle = (checked: boolean) => {
    console.log("Trim toggle:", checked);
    // TODO: Implement trim functionality
  };

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
        className="bg-white w-full max-w-xl rounded-2xl shadow-2xl overflow-hidden flex flex-col max-h-[80vh] m-4"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="px-8 pt-8 pb-4 flex justify-between items-center">
          <h2 id="modal-title" className="text-2xl font-bold text-primary">
            Download Options
          </h2>
          <button
            onClick={onClose}
            className="text-slate-400 hover:text-slate-600 transition-colors"
            aria-label="Close modal"
          >
            <X size={24} />
          </button>
        </div>

        {/* Scrollable Content */}
        <div className="px-8 pb-8 space-y-8 overflow-y-auto">
          {/* Video Preview */}
          <div className="flex gap-4 p-4 bg-slate-50 rounded-custom border border-slate-100">
            <div className="relative w-32 h-20 bg-slate-200 rounded-md overflow-hidden flex-shrink-0">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                alt="Video thumbnail"
                className="w-full h-full object-cover"
                src="https://lh3.googleusercontent.com/aida-public/AB6AXuDVlggxhxQJsaKSnIhJ2PVlvkYufAGie6a3GFGGDfm6ptIItwjdMteHe5jSssxQp4z7rsPw52eRgzb3hdEEl_-SnEgtLjsjZJ4jwYbVDp5yByWVPGcMwgbQCr-O37-HPe8T0tNpUgvdiTQ6PSB2lC6yTR19kVbeYtN24TY8TSQ56PGL0GO7_XJo2agXmijaCUpkGWlLlDrBcAv9F6JbBT2SgAmXwg-DEmX2MNiha_nYYWuZjz5UkyTEGSt6KybAmR_T9IegYpnY"
              />
              <div className="absolute bottom-1 right-1 bg-black/60 text-white text-[10px] px-1 rounded">
                12:45
              </div>
            </div>
            <div className="flex flex-col justify-center">
              <div className="flex items-center gap-2 mb-1">
                <span className="bg-red-100 text-red-600 text-[10px] font-bold px-1.5 py-0.5 rounded flex items-center gap-1">
                  <YoutubeLogo size={12} weight="fill" />
                  YouTube
                </span>
                <span className="text-slate-400 text-[10px]">
                  Added 2 mins ago
                </span>
              </div>
              <h3 className="font-semibold text-slate-800 leading-tight">
                How to Design Sophisticated UIs
              </h3>
              <p className="text-xs text-slate-500 mt-1">
                Channel: DesignMaster Pro
              </p>
            </div>
          </div>

          {/* Download Resolution Options */}
          <div>
            <h4 className="text-sm font-bold text-slate-400 uppercase tracking-wider mb-4">
              Direct Download
            </h4>
            <div className="grid grid-cols-3 gap-3">
              {RESOLUTION_OPTIONS.map((option) => (
                <button
                  key={option.quality}
                  className={`flex flex-col items-center justify-center p-4 border-2 rounded-custom transition-all group ${
                    option.recommended
                      ? "border-primary/20 bg-primary/5"
                      : "border-slate-100 hover:border-primary hover:bg-primary/5"
                  }`}
                >
                  <span
                    className={`font-bold text-lg ${
                      option.recommended
                        ? "text-primary"
                        : "text-slate-700 group-hover:text-primary"
                    }`}
                  >
                    {option.quality}
                  </span>
                  <span
                    className={`text-[10px] font-medium ${
                      option.recommended
                        ? "text-primary/60"
                        : "text-slate-400 group-hover:text-primary/60"
                    }`}
                  >
                    {option.format} • {option.size}
                  </span>
                </button>
              ))}
            </div>
          </div>

          {/* Trim Video Feature */}
          <div className="flex items-center justify-between p-4 bg-slate-50 rounded-custom border border-slate-100">
            <div className="flex items-center gap-3">
              <div className="w-10 h-10 bg-white shadow-sm rounded-full flex items-center justify-center text-primary">
                <Scissors size={20} />
              </div>
              <div>
                <p className="font-semibold text-slate-800 text-sm">
                  Trim Video
                </p>
                <p className="text-xs text-slate-500">
                  Download only a specific part
                </p>
              </div>
            </div>
            <label className="relative inline-flex items-center cursor-pointer">
              <input
                type="checkbox"
                className="sr-only peer"
                onChange={(e) => handleTrimToggle(e.target.checked)}
              />
              <div className="w-11 h-6 bg-slate-200 peer-focus:outline-none rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary" />
            </label>
          </div>

          {/* Action Buttons */}
          <div className="pt-4 border-t border-slate-100">
            <button
              onClick={handleDownload}
              className="w-full bg-primary text-white py-4 rounded-custom font-bold text-lg hover:bg-primary-dark transition-all shadow-lg shadow-primary/20 flex items-center justify-center gap-3"
            >
              <Download size={24} weight="bold" />
              Download MP4
            </button>
            <p className="text-center text-[11px] text-slate-400 mt-4 leading-relaxed px-12">
              By clicking download, you agree to our Terms of Service. Please
              ensure you have the rights to download this content.
            </p>
          </div>
        </div>
      </div>
    </div>
  );

  return createPortal(modalContent, document.body);
}
