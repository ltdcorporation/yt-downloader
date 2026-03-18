import { YoutubeLogo, TiktokLogo, InstagramLogo, XLogo } from "@phosphor-icons/react";

export default function PlatformIcons() {
  return (
    <div className="flex flex-wrap justify-center items-center gap-6 sm:gap-8 pt-4 px-4">
      <div className="flex flex-col items-center gap-2 group cursor-pointer min-w-[72px]">
        <div className="w-12 h-12 flex items-center justify-center rounded-full bg-white dark:bg-slate-900 text-primary border border-primary/10 shadow-sm group-hover:shadow-md group-hover:scale-110 transition-all">
          <YoutubeLogo size={24} weight="fill" />
        </div>
        <span className="text-xs font-semibold text-slate-400 group-hover:text-primary transition-colors whitespace-nowrap">
          YouTube
        </span>
      </div>
      <div className="flex flex-col items-center gap-2 group cursor-pointer min-w-[72px]">
        <div className="w-12 h-12 flex items-center justify-center rounded-full bg-white dark:bg-slate-900 text-primary border border-primary/10 shadow-sm group-hover:shadow-md group-hover:scale-110 transition-all">
          <TiktokLogo size={24} weight="fill" />
        </div>
        <span className="text-xs font-semibold text-slate-400 group-hover:text-primary transition-colors whitespace-nowrap">
          TikTok
        </span>
      </div>
      <div className="flex flex-col items-center gap-2 group cursor-pointer min-w-[72px]">
        <div className="w-12 h-12 flex items-center justify-center rounded-full bg-white dark:bg-slate-900 text-primary border border-primary/10 shadow-sm group-hover:shadow-md group-hover:scale-110 transition-all">
          <InstagramLogo size={24} weight="fill" />
        </div>
        <span className="text-xs font-semibold text-slate-400 group-hover:text-primary transition-colors whitespace-nowrap">
          Instagram
        </span>
      </div>
      <div className="flex flex-col items-center gap-2 group cursor-pointer min-w-[72px]">
        <div className="w-12 h-12 flex items-center justify-center rounded-full bg-white dark:bg-slate-900 text-primary border border-primary/10 shadow-sm group-hover:shadow-md group-hover:scale-110 transition-all">
          <XLogo size={24} weight="fill" />
        </div>
        <span className="text-xs font-semibold text-slate-400 group-hover:text-primary transition-colors whitespace-nowrap">
          Twitter (X)
        </span>
      </div>
    </div>
  );
}
