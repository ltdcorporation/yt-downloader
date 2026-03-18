import { VideoCamera } from "@phosphor-icons/react";

export default function Footer() {
  return (
    <footer className="px-4 sm:px-6 py-10 lg:px-20 border-t border-primary/10 mt-16">
      <div className="max-w-[1200px] mx-auto flex flex-col items-center gap-6">
        {/* Copyright */}
        <div className="flex items-center gap-2">
          <VideoCamera size={24} weight="fill" className="text-primary" />
          <span className="text-sm font-bold text-slate-500">© 2024 QuickSnap. All rights reserved.</span>
        </div>

        {/* Legal Links - Horizontal Scrollable on Mobile */}
        <div className="relative w-full max-w-md">
          <div className="flex gap-6 overflow-x-auto scrollbar-hide -mx-4 px-4 sm:mx-0 sm:px-0 sm:overflow-visible sm:gap-8">
            <a className="text-slate-400 hover:text-primary text-xs font-medium transition-colors whitespace-nowrap" href="#">
              Privacy Policy
            </a>
            <a className="text-slate-400 hover:text-primary text-xs font-medium transition-colors whitespace-nowrap" href="#">
              Terms of Service
            </a>
            <a className="text-slate-400 hover:text-primary text-xs font-medium transition-colors whitespace-nowrap" href="#">
              Cookie Settings
            </a>
          </div>
          {/* Scroll indicators for mobile */}
          <div className="absolute inset-y-0 right-0 w-8 bg-gradient-to-l from-white dark:from-slate-950 to-transparent pointer-events-none sm:hidden" />
        </div>
      </div>
    </footer>
  );
}
