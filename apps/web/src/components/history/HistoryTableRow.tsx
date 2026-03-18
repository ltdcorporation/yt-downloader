import { CloudArrowDown, Trash } from "@phosphor-icons/react";
import type { DownloadHistory } from "@/data/history-sample-data";
import { PLATFORM_COLORS } from "@/data/history-sample-data";

interface HistoryTableRowProps {
  item: DownloadHistory;
  onDownloadAgain: (id: string) => void;
  onDelete: (id: string) => void;
}

export default function HistoryTableRow({
  item,
  onDownloadAgain,
  onDelete,
}: HistoryTableRowProps) {
  return (
    <tr className="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
      <td className="px-6 py-4">
        <div className="flex items-center gap-4">
          <div
            className="bg-center bg-no-repeat aspect-video bg-cover rounded-lg w-24 h-14 shadow-sm border border-slate-100 dark:border-slate-800"
            style={{ backgroundImage: `url("${item.thumbnail}")` }}
            role="img"
            aria-label={`Thumbnail for ${item.title}`}
          />
          <div className="flex flex-col">
            <p className="text-slate-900 dark:text-slate-100 text-sm font-semibold line-clamp-1">
              {item.title}
            </p>
            <p className="text-slate-400 text-xs">{item.quality}</p>
          </div>
        </div>
      </td>
      <td className="px-6 py-4">
        <div className="flex items-center gap-2">
          <span
            className={`px-2 py-1 rounded text-[10px] font-bold uppercase ${
              PLATFORM_COLORS[item.platform]
            }`}
          >
            {item.platform}
          </span>
        </div>
      </td>
      <td className="px-6 py-4 text-slate-500 dark:text-slate-400 text-sm">
        {item.size}
      </td>
      <td className="px-6 py-4 text-slate-500 dark:text-slate-400 text-sm">
        {item.date}
      </td>
      <td className="px-6 py-4">
        <div className="flex items-center justify-end gap-3">
          <button
            className="flex items-center gap-2 px-3 py-1.5 bg-primary text-white text-xs font-bold rounded-lg hover:bg-primary/90 transition-all"
            onClick={() => onDownloadAgain(item.id)}
          >
            <CloudArrowDown size={18} weight="fill" />
            Download Again
          </button>
          <button
            className="p-1.5 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-all"
            onClick={() => onDelete(item.id)}
            aria-label="Delete"
          >
            <Trash size={20} weight="fill" />
          </button>
        </div>
      </td>
    </tr>
  );
}
