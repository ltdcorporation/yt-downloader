type Platform = "all" | "youtube" | "tiktok" | "instagram";

interface HistoryTabsProps {
  activeTab: Platform;
  onChange: (platform: Platform) => void;
}

const TABS: { id: Platform; label: string }[] = [
  { id: "all", label: "All Downloads" },
  { id: "youtube", label: "YouTube" },
  { id: "tiktok", label: "TikTok" },
  { id: "instagram", label: "Instagram" },
];

export default function HistoryTabs({ activeTab, onChange }: HistoryTabsProps) {
  return (
    <div className="flex border-b border-slate-200 dark:border-slate-800 gap-8 overflow-x-auto no-scrollbar">
      {TABS.map((tab) => (
        <button
          key={tab.id}
          className={`flex items-center justify-center border-b-2 pb-3 px-2 whitespace-nowrap transition-colors ${
            activeTab === tab.id
              ? "border-primary text-primary"
              : "border-transparent text-slate-500 dark:text-slate-400 hover:text-slate-700"
          }`}
          onClick={() => onChange(tab.id)}
        >
          <span
            className={`text-sm ${activeTab === tab.id ? "font-bold" : "font-medium"}`}
          >
            {tab.label}
          </span>
        </button>
      ))}
    </div>
  );
}
