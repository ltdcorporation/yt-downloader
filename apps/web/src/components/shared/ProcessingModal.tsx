"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import {
  CircleNotch,
  DownloadSimple,
  CheckCircle,
  WarningCircle,
  X,
} from "@phosphor-icons/react";

interface ProcessingModalProps {
  isOpen: boolean;
  title?: string;
  status?: "processing" | "completed" | "failed";
  heading?: string;
  description?: string;
  primaryActionLabel?: string;
  onPrimaryAction?: () => void;
  secondaryActionLabel?: string;
  onSecondaryAction?: () => void;
  onClose?: () => void;
}

export default function ProcessingModal({
  isOpen,
  title,
  status = "processing",
  heading,
  description,
  primaryActionLabel,
  onPrimaryAction,
  secondaryActionLabel,
  onSecondaryAction,
  onClose,
}: ProcessingModalProps) {
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

  if (!isOpen || !isMounted) return null;

  const fallbackHeading =
    status === "completed"
      ? "Selesai"
      : status === "failed"
        ? "Gagal"
        : "Memproses...";

  const fallbackDescription =
    status === "completed"
      ? "File kamu sudah siap."
      : status === "failed"
        ? "Terjadi masalah saat memproses permintaan."
        : "Kami sedang menyiapkan file untuk kamu.";

  const modalContent = (
    <div className="fixed inset-0 z-[10001] flex items-center justify-center bg-black/70 backdrop-blur-md p-4">
      <div className="relative bg-white dark:bg-slate-900 w-full max-w-sm rounded-3xl shadow-2xl p-10 text-center animate-in zoom-in duration-300">
        {onClose ? (
          <button
            type="button"
            onClick={onClose}
            className="absolute top-4 right-4 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
            aria-label="Close"
          >
            <X size={20} />
          </button>
        ) : null}

        {status === "processing" ? (
          <>
            <div className="relative w-20 h-20 mx-auto mb-6">
              <CircleNotch size={80} className="text-primary animate-spin" />
              <div className="absolute inset-0 flex items-center justify-center">
                <DownloadSimple size={32} className="text-primary/40" />
              </div>
            </div>
            <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
              {heading || fallbackHeading}
            </h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              {description || fallbackDescription}
            </p>
            <div className="mt-8 h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
              <div className="h-full bg-primary animate-progress rounded-full" />
            </div>
          </>
        ) : status === "completed" ? (
          <>
            <div className="w-20 h-20 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mx-auto mb-6 animate-in zoom-in">
              <CheckCircle size={48} weight="fill" className="text-green-500" />
            </div>
            <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
              {heading || fallbackHeading}
            </h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              {description || fallbackDescription}
            </p>
          </>
        ) : (
          <>
            <div className="w-20 h-20 bg-rose-100 dark:bg-rose-900/30 rounded-full flex items-center justify-center mx-auto mb-6 animate-in zoom-in">
              <WarningCircle
                size={48}
                weight="fill"
                className="text-rose-600"
              />
            </div>
            <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
              {heading || fallbackHeading}
            </h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              {description || fallbackDescription}
            </p>
          </>
        )}

        {primaryActionLabel && onPrimaryAction ? (
          <div className="mt-8 flex flex-col gap-2">
            <button
              type="button"
              onClick={onPrimaryAction}
              className="inline-flex items-center justify-center rounded-xl bg-primary px-4 py-3 font-bold text-white shadow-lg shadow-primary/20 hover:brightness-105 transition-all"
            >
              {primaryActionLabel}
            </button>

            {secondaryActionLabel && onSecondaryAction ? (
              <button
                type="button"
                onClick={onSecondaryAction}
                className="inline-flex items-center justify-center rounded-xl bg-slate-100 dark:bg-slate-800 px-4 py-3 font-bold text-slate-700 dark:text-slate-200 hover:bg-slate-200 dark:hover:bg-slate-700 transition-all"
              >
                {secondaryActionLabel}
              </button>
            ) : null}
          </div>
        ) : null}

        <p className="mt-10 text-[10px] text-slate-400 uppercase tracking-widest font-bold">
          {title || "Video Download"}
        </p>
      </div>

      <style jsx>{`
        @keyframes progress {
          0% {
            width: 0%;
          }
          100% {
            width: 100%;
          }
        }
        .animate-progress {
          animation: progress 3s ease-in-out infinite;
        }
      `}</style>
    </div>
  );

  return createPortal(modalContent, document.body);
}
