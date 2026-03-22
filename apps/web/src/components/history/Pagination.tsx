interface PaginationProps {
  currentPage: number;
  pageSize: number;
  currentCount: number;
  totalItems?: number | null;
  hasNextPage: boolean;
  hasPrevPage: boolean;
  isLoading?: boolean;
  onNext: () => void;
  onPrevious: () => void;
}

export default function Pagination({
  currentPage,
  pageSize,
  currentCount,
  totalItems,
  hasNextPage,
  hasPrevPage,
  isLoading = false,
  onNext,
  onPrevious,
}: PaginationProps) {
  const hasKnownTotal = typeof totalItems === "number" && totalItems >= 0;
  const startItem = currentCount === 0 ? 0 : (currentPage - 1) * pageSize + 1;
  const endItem = currentCount === 0 ? 0 : startItem + currentCount - 1;

  return (
    <div className="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
      <p className="text-slate-500 dark:text-slate-400 text-xs">
        {hasKnownTotal
          ? `Showing ${startItem} to ${endItem} of ${totalItems} downloads`
          : `Page ${currentPage} · Showing ${currentCount} items`}
      </p>

      <div className="flex items-center gap-2">
        <button
          className="px-3 py-1 bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 transition-colors disabled:opacity-50"
          disabled={!hasPrevPage || isLoading}
          onClick={onPrevious}
        >
          Previous
        </button>

        <span className="px-3 py-1 rounded bg-primary/10 text-primary text-xs font-bold">
          Page {currentPage}
        </span>

        <button
          className="px-3 py-1 bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 transition-colors disabled:opacity-50"
          disabled={!hasNextPage || isLoading}
          onClick={onNext}
        >
          Next
        </button>
      </div>
    </div>
  );
}
