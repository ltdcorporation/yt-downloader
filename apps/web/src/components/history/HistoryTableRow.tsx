import { CloudArrowDown, Trash } from "@phosphor-icons/react";
import type { HistoryAttemptStatus, HistoryTableItem } from "./types";

interface HistoryTableRowProps {
  item: HistoryTableItem;
  onDownloadAgain: (id: string) => void;
  onDelete: (id: string) => void;
  isBusy?: boolean;
}

const PLATFORM_COLORS: Record<HistoryTableItem["platform"], string> = {
  youtube: "bg-red-100 text-red-600",
  tiktok: "bg-slate-900 text-white",
  instagram: "bg-pink-100 text-pink-600",
  x: "bg-slate-800 text-white",
};

const STATUS_COLORS: Record<HistoryAttemptStatus, string> = {
  queued: "bg-amber-100 text-amber-700",
  processing: "bg-sky-100 text-sky-700",
  done: "bg-emerald-100 text-emerald-700",
  failed: "bg-rose-100 text-rose-700",
  expired: "bg-slate-200 text-slate-700",
};

const STATUS_LABELS: Record<HistoryAttemptStatus, string> = {
  queued: "Queued",
  processing: "Processing",
  done: "Done",
  failed: "Failed",
  expired: "Expired",
};

export default function HistoryTableRow({
  item,
  onDownloadAgain,
  onDelete,
  isBusy = false,
}: HistoryTableRowProps) {
  const thumbnailStyle = item.thumbnail
    ? { backgroundImage: `url("${item.thumbnail}")` }
    : undefined;

  return (
    <tr className="hover:bg-slate-50 dark:hover:bg-slate-800/30 transition-colors">
      <td className="px-6 py-4">
        <div className="flex items-center gap-4">
          <div
            className="bg-center bg-no-repeat aspect-video bg-cover rounded-lg w-24 h-14 shadow-sm border border-slate-100 dark:border-slate-800 bg-slate-100 dark:bg-slate-800 flex items-center justify-center text-[10px] text-slate-400"
            style={thumbnailStyle}
            role="img"
            aria-label={`Thumbnail for ${item.title}`}
          >
            {!item.thumbnail ? "No image" : null}
          </div>
          <div className="flex flex-col min-w-0">
            <p className="text-slate-900 dark:text-slate-100 text-sm font-semibold line-clamp-1">
              {item.title}
            </p>
            <div className="flex items-center gap-2 mt-0.5">
              <p className="text-slate-400 text-xs">{item.quality}</p>
              <span
                className={`px-1.5 py-0.5 rounded text-[10px] font-bold uppercase ${STATUS_COLORS[item.status]}`}
              >
                {STATUS_LABELS[item.status]}
              </span>
            </div>
          </div>
        </div>
      </td>

      <td className="px-6 py-4">
        <span
          className={`px-2 py-1 rounded text-[10px] font-bold uppercase ${PLATFORM_COLORS[item.platform]}`}
        >
          {item.platform}
        </span>
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
            className="flex items-center gap-2 px-3 py-1.5 bg-primary text-white text-xs font-bold rounded-lg hover:bg-primary/90 transition-all disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={() => onDownloadAgain(item.id)}
            disabled={isBusy}
          >
            <CloudArrowDown size={18} weight="fill" />
            {isBusy ? "Working..." : "Download Again"}
          </button>

          <button
            className="p-1.5 text-slate-400 hover:text-red-500 hover:bg-red-50 dark:hover:bg-red-900/20 rounded-lg transition-all disabled:opacity-60 disabled:cursor-not-allowed"
            onClick={() => onDelete(item.id)}
            aria-label="Delete"
            disabled={isBusy}
          >
            <Trash size={20} weight="fill" />
          </button>
        </div>
      </td>
    </tr>
  );
}
