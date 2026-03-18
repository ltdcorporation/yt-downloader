interface PaginationProps {
  currentPage: number;
  totalPages: number;
  totalItems: number;
  itemsPerPage?: number;
}

export default function Pagination({
  currentPage,
  totalPages,
  totalItems,
  itemsPerPage = 10,
}: PaginationProps) {
  const startItem = (currentPage - 1) * itemsPerPage + 1;
  const endItem = Math.min(currentPage * itemsPerPage, totalItems);

  return (
    <div className="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 flex items-center justify-between">
      <p className="text-slate-500 dark:text-slate-400 text-xs">
        Showing {startItem} to {endItem} of {totalItems} downloads
      </p>
      <div className="flex gap-2">
        <button
          className="px-3 py-1 bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 transition-colors disabled:opacity-50"
          disabled={currentPage === 1}
        >
          Previous
        </button>
        {Array.from({ length: totalPages }, (_, i) => i + 1).map((page) => (
          <button
            key={page}
            className={`px-3 py-1 rounded text-xs font-medium transition-colors ${
              page === currentPage
                ? "bg-primary text-white hover:bg-primary/90"
                : "bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 text-slate-600 dark:text-slate-300 hover:bg-slate-50"
            }`}
          >
            {page}
          </button>
        ))}
        <button
          className="px-3 py-1 bg-white dark:bg-slate-700 border border-slate-200 dark:border-slate-600 rounded text-xs font-medium text-slate-600 dark:text-slate-300 hover:bg-slate-50 transition-colors disabled:opacity-50"
          disabled={currentPage === totalPages}
        >
          Next
        </button>
      </div>
    </div>
  );
}
