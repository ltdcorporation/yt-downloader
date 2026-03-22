import { Download, Database, Calendar } from "@phosphor-icons/react";
import type { HistoryStatCard } from "./types";

interface StatsCardsProps {
  stats: HistoryStatCard[];
}

const ICON_MAP: Record<HistoryStatCard["icon"], JSX.Element> = {
  download: <Download size={24} weight="fill" />,
  database: <Database size={24} weight="fill" />,
  calendar: <Calendar size={24} weight="fill" />,
};

export default function StatsCards({ stats }: StatsCardsProps) {
  return (
    <div className="mt-10 grid grid-cols-1 md:grid-cols-3 gap-6">
      {stats.map((stat, index) => (
        <div
          key={`${stat.label}-${index}`}
          className="bg-primary/10 border border-primary/20 rounded-xl p-5 flex items-center gap-4"
        >
          <div className="size-12 rounded-full bg-primary text-white flex items-center justify-center">
            {ICON_MAP[stat.icon]}
          </div>
          <div>
            <p className="text-primary text-xs font-bold uppercase tracking-wide">
              {stat.label}
            </p>
            <p className="text-slate-900 dark:text-slate-100 text-2xl font-bold">
              {stat.value}
            </p>
          </div>
        </div>
      ))}
    </div>
  );
}
