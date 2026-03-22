import type { HistoryTableItem } from "./types";
import HistoryTableRow from "./HistoryTableRow";

interface HistoryTableProps {
  data: HistoryTableItem[];
  onDownloadAgain: (id: string) => void;
  onDelete: (id: string) => void;
  isLoading?: boolean;
  busyRowId?: string | null;
}

export default function HistoryTable({
  data,
  onDownloadAgain,
  onDelete,
  isLoading = false,
  busyRowId = null,
}: HistoryTableProps) {
  return (
    <div className="overflow-hidden rounded-xl border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm">
      <div className="overflow-x-auto">
        <table className="w-full text-left border-collapse">
          <thead>
            <tr className="bg-slate-50 dark:bg-slate-800/50">
              <th className="px-6 py-4 text-slate-600 dark:text-slate-300 text-xs font-bold uppercase tracking-wider">
                Video
              </th>
              <th className="px-6 py-4 text-slate-600 dark:text-slate-300 text-xs font-bold uppercase tracking-wider">
                Platform
              </th>
              <th className="px-6 py-4 text-slate-600 dark:text-slate-300 text-xs font-bold uppercase tracking-wider">
                Size
              </th>
              <th className="px-6 py-4 text-slate-600 dark:text-slate-300 text-xs font-bold uppercase tracking-wider">
                Date
              </th>
              <th className="px-6 py-4 text-slate-600 dark:text-slate-300 text-xs font-bold uppercase tracking-wider text-right">
                Actions
              </th>
            </tr>
          </thead>

          <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
            {isLoading
              ? Array.from({ length: 4 }).map((_, index) => (
                  <tr key={`history-skeleton-${index}`}>
                    <td colSpan={5} className="px-6 py-4">
                      <div className="h-14 rounded-lg bg-slate-100 dark:bg-slate-800 animate-pulse" />
                    </td>
                  </tr>
                ))
              : null}

            {!isLoading && data.length === 0 ? (
              <tr>
                <td
                  colSpan={5}
                  className="px-6 py-12 text-center text-sm text-slate-500 dark:text-slate-400"
                >
                  No history found for the current filter.
                </td>
              </tr>
            ) : null}

            {!isLoading
              ? data.map((item) => (
                  <HistoryTableRow
                    key={item.id}
                    item={item}
                    onDownloadAgain={onDownloadAgain}
                    onDelete={onDelete}
                    isBusy={busyRowId === item.id}
                  />
                ))
              : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}
