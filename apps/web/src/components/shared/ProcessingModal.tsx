"use client";

import { useEffect, useState } from "react";
import { createPortal } from "react-dom";
import { CircleNotch, DownloadSimple, CheckCircle } from "@phosphor-icons/react";

interface ProcessingModalProps {
  isOpen: boolean;
  title?: string;
}

export default function ProcessingModal({ isOpen, title }: ProcessingModalProps) {
  const [isMounted, setIsMounted] = useState(false);
  const [status, setStatus] = useState<"processing" | "completed">("processing");

  useEffect(() => {
    setIsMounted(true);
  }, []);

  useEffect(() => {
    if (isOpen) {
      setStatus("processing");
      document.body.style.overflow = "hidden";
    } else {
      document.body.style.overflow = "";
    }
    return () => {
      document.body.style.overflow = "";
    };
  }, [isOpen]);

  // Mock completion after 3 seconds
  useEffect(() => {
    if (isOpen && status === "processing") {
      const timer = setTimeout(() => {
        setStatus("completed");
      }, 3000);
      return () => clearTimeout(timer);
    }
  }, [isOpen, status]);

  if (!isOpen || !isMounted) return null;

  const modalContent = (
    <div className="fixed inset-0 z-[10001] flex items-center justify-center bg-black/70 backdrop-blur-md p-4">
      <div className="bg-white dark:bg-slate-900 w-full max-w-sm rounded-3xl shadow-2xl p-10 text-center animate-in zoom-in duration-300">
        {status === "processing" ? (
          <>
            <div className="relative w-20 h-20 mx-auto mb-6">
              <CircleNotch size={80} className="text-primary animate-spin" />
              <div className="absolute inset-0 flex items-center justify-center">
                <DownloadSimple size={32} className="text-primary/40" />
              </div>
            </div>
            <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
              Processing...
            </h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              We are preparing your high-quality file for download.
            </p>
            <div className="mt-8 h-1.5 w-full bg-slate-100 dark:bg-slate-800 rounded-full overflow-hidden">
              <div className="h-full bg-primary animate-progress rounded-full" style={{ width: "60%" }}></div>
            </div>
          </>
        ) : (
          <>
            <div className="w-20 h-20 bg-green-100 dark:bg-green-900/30 rounded-full flex items-center justify-center mx-auto mb-6 animate-in zoom-in">
              <CheckCircle size={48} weight="fill" className="text-green-500" />
            </div>
            <h3 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
              Done!
            </h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm">
              Your download should start automatically now.
            </p>
          </>
        )}
        
        <p className="mt-10 text-[10px] text-slate-400 uppercase tracking-widest font-bold">
          {title || "Video Download"}
        </p>
      </div>
      
      <style jsx>{`
        @keyframes progress {
          0% { width: 0%; }
          100% { width: 100%; }
        }
        .animate-progress {
          animation: progress 3s ease-in-out forwards;
        }
      `}</style>
    </div>
  );

  return createPortal(modalContent, document.body);
}
