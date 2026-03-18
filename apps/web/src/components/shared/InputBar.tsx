"use client";

import { useState } from "react";
import { LinkSimpleHorizontal, Clipboard, DownloadSimple } from "@phosphor-icons/react";
import DownloadModal from "./DownloadModal";

export default function InputBar() {
  const [url, setUrl] = useState("");
  const [isModalOpen, setIsModalOpen] = useState(false);
  const [isLoading, setIsLoading] = useState(false);

  const handlePaste = async () => {
    try {
      const text = await navigator.clipboard.readText();
      setUrl(text);
      if (text.trim()) {
        handleProcess();
      }
    } catch (err) {
      console.error("Failed to paste:", err);
    }
  };

  const handleProcess = () => {
    if (!url.trim()) return;
    setIsLoading(true);
    // Simulate loading before opening modal
    setTimeout(() => {
      setIsLoading(false);
      setIsModalOpen(true);
    }, 1500);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
  };

  const handleInputChange = (e: React.ChangeEvent<HTMLInputElement>) => {
    setUrl(e.target.value);
  };

  const handleInputFocus = (e: React.FocusEvent<HTMLInputElement>) => {
    if (e.target.value.trim()) {
      setIsModalOpen(true);
    }
  };

  const handleInputKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "Enter") {
      handleProcess();
    }
  };

  return (
    <>
      <div className="relative w-full max-w-[640px] mx-auto group">
        <div className="flex flex-col gap-4">
          {/* Input Bar */}
          <div className="flex items-center bg-white dark:bg-slate-900 border border-primary/20 rounded-xl p-2 shadow-xl shadow-primary/5 focus-within:ring-2 focus-within:ring-primary/30 transition-all">
            <LinkSimpleHorizontal
              size={20}
              weight="fill"
              className="text-slate-400 ml-3 flex-shrink-0"
            />
            <input
              className="flex-1 bg-transparent border-none focus:ring-0 text-slate-900 dark:text-slate-100 placeholder:text-slate-400 px-4 py-3 text-base md:text-lg"
              placeholder="Paste your video URL here..."
              type="text"
              value={url}
              onChange={handleInputChange}
              onFocus={handleInputFocus}
              onKeyDown={handleInputKeyDown}
            />
            <button
              onClick={handlePaste}
              className="flex items-center gap-2 bg-primary text-white px-4 sm:px-6 py-3 rounded-lg font-bold hover:brightness-105 transition-all shadow-md flex-shrink-0"
            >
              <Clipboard size={20} weight="fill" />
              <span className="hidden sm:inline">Paste</span>
            </button>
          </div>

          {/* Process Button - Stacked below input on mobile */}
          <button
            onClick={handleProcess}
            disabled={!url.trim() || isLoading}
            className="w-full bg-primary text-white py-4 rounded-xl font-bold text-lg hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 flex items-center justify-center gap-3 disabled:opacity-50 disabled:cursor-not-allowed"
          >
            {isLoading ? (
              <>
                <span className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
                <span>Processing...</span>
              </>
            ) : (
              <>
                <DownloadSimple size={24} weight="fill" />
                <span>Download / Process</span>
              </>
            )}
          </button>
        </div>
      </div>

      <DownloadModal
        isOpen={isModalOpen}
        onClose={handleCloseModal}
        videoUrl={url}
        isLoading={isLoading}
      />
    </>
  );
}
