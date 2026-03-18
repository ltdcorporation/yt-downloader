import type { DownloadHistory } from "@/data/history-sample-data";
import HistoryTableRow from "./HistoryTableRow";

interface HistoryTableProps {
  data: DownloadHistory[];
  onDownloadAgain: (id: string) => void;
  onDelete: (id: string) => void;
}

export default function HistoryTable({
  data,
  onDownloadAgain,
  onDelete,
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
            {data.map((item) => (
              <HistoryTableRow
                key={item.id}
                item={item}
                onDownloadAgain={onDownloadAgain}
                onDelete={onDelete}
              />
            ))}
          </tbody>
        </table>
      </div>
    </div>
  );
}
