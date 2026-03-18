import InputBar from "./InputBar";
import PlatformIcons from "./PlatformIcons";

export default function HeroSection() {
  return (
    <main className="flex-1 flex flex-col items-center justify-center px-4 sm:px-6 py-12 lg:py-24 animate-fade-in">
      <div className="max-w-[800px] w-full text-center space-y-8">
        {/* Title */}
        <div className="space-y-4 px-2 sm:px-0 animate-slide-up">
          <h1 className="text-primary text-3xl sm:text-4xl md:text-5xl lg:text-7xl font-black tracking-tight">
            QuickSnap
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-base sm:text-lg md:text-xl font-medium max-w-[600px] mx-auto px-4 sm:px-0 leading-relaxed">
            The cleanest way to download high-quality videos from your favorite social platforms.
          </p>
        </div>

        {/* Input Bar */}
        <div className="animate-slide-up" style={{ animationDelay: "0.1s" }}>
          <InputBar />
        </div>

        {/* Platform Icons */}
        <div className="animate-slide-up" style={{ animationDelay: "0.2s" }}>
          <PlatformIcons />
        </div>
      </div>
    </main>
  );
}
