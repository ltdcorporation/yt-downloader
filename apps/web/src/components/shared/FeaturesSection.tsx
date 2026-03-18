import { HighDefinition, Lightning, DropSlash } from "@phosphor-icons/react";

export default function FeaturesSection() {
  return (
    <div className="flex justify-center w-full px-4 sm:px-6" id="features">
      <div className="grid grid-cols-1 md:grid-cols-3 gap-6 md:gap-8 max-w-[960px] w-full">
        <div className="flex flex-col gap-4 rounded-xl border border-primary/10 bg-white/50 dark:bg-slate-900/50 p-6 sm:p-8 backdrop-blur-sm">
          <div className="text-primary bg-primary/10 w-12 h-12 flex items-center justify-center rounded-lg">
            <HighDefinition size={24} weight="fill" />
          </div>
          <div className="flex flex-col gap-1">
            <h3 className="text-slate-900 dark:text-slate-100 text-lg font-bold">Ultra HD Quality</h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm leading-relaxed">
              Save videos in original resolution, from 720p up to stunning 4K clarity.
            </p>
          </div>
        </div>
        <div className="flex flex-col gap-4 rounded-xl border border-primary/10 bg-white/50 dark:bg-slate-900/50 p-6 sm:p-8 backdrop-blur-sm">
          <div className="text-primary bg-primary/10 w-12 h-12 flex items-center justify-center rounded-lg">
            <Lightning size={24} weight="fill" />
          </div>
          <div className="flex flex-col gap-1">
            <h3 className="text-slate-900 dark:text-slate-100 text-lg font-bold">Lightning Fast</h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm leading-relaxed">
              No waiting around. Our high-performance servers process your links in real-time.
            </p>
          </div>
        </div>
        <div className="flex flex-col gap-4 rounded-xl border border-primary/10 bg-white/50 dark:bg-slate-900/50 p-6 sm:p-8 backdrop-blur-sm">
          <div className="text-primary bg-primary/10 w-12 h-12 flex items-center justify-center rounded-lg">
            <DropSlash size={24} weight="fill" />
          </div>
          <div className="flex flex-col gap-1">
            <h3 className="text-slate-900 dark:text-slate-100 text-lg font-bold">No Watermarks</h3>
            <p className="text-slate-500 dark:text-slate-400 text-sm leading-relaxed">
              Get clean, professional video files without any platform branding or overlays.
            </p>
          </div>
        </div>
      </div>
    </div>
  );
}
