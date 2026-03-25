"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import Navbar from "@/components/shared/Navbar";
import Footer from "@/components/shared/Footer";
import SearchBar from "@/components/history/SearchBar";
import HistoryTabs from "@/components/history/HistoryTabs";
import HistoryTable from "@/components/history/HistoryTable";
import Pagination from "@/components/history/Pagination";
import StatsCards from "@/components/history/StatsCards";
import type {
  HistoryStatCard,
  HistoryTableItem,
  HistoryTabPlatform,
} from "@/components/history/types";
import {
  api,
  APIError,
  type HistoryListItem,
  type HistoryRequestKind,
  type HistoryStatsResponse,
} from "@/lib/api";
import { useAuthStore } from "@/store";
import { Lock } from "@phosphor-icons/react";

const PAGE_SIZE = 10;
const MP3_JOB_POLL_INTERVAL_MS = 2000;
const MP3_JOB_POLL_TIMEOUT_MS = 2 * 60 * 1000;

export default function HistoryPage() {
  const { currentUser, isAuthChecking, setLoginModalOpen, setCurrentUser } =
    useAuthStore();
  const router = useRouter();

  const [searchTerm, setSearchTerm] = useState("");
  const debouncedSearchTerm = useDebouncedValue(searchTerm, 350);
  const [activeTab, setActiveTab] = useState<HistoryTabPlatform>("all");

  const [historyItems, setHistoryItems] = useState<HistoryListItem[]>([]);
  const [stats, setStats] = useState<HistoryStatsResponse | null>(null);

  const [isHistoryLoading, setIsHistoryLoading] = useState(false);
  const [isStatsLoading, setIsStatsLoading] = useState(false);

  const [historyError, setHistoryError] = useState("");
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");

  const [busyRowId, setBusyRowId] = useState<string | null>(null);

  const [currentPageIndex, setCurrentPageIndex] = useState(0);
  const [pageCursors, setPageCursors] = useState<(string | null)[]>([null]);
  const [nextCursor, setNextCursor] = useState<string | null>(null);
  const [hasMore, setHasMore] = useState(false);

  const [refreshTick, setRefreshTick] = useState(0);

  const currentCursor = pageCursors[currentPageIndex] ?? null;
  const hasPrevPage = currentPageIndex > 0;
  const hasActiveFilters =
    activeTab !== "all" || debouncedSearchTerm.trim().length > 0;

  useEffect(() => {
    if (currentUser) {
      return;
    }

    setSearchTerm("");
    setActiveTab("all");
    setHistoryItems([]);
    setStats(null);
    setHistoryError("");
    setActionError("");
    setActionMessage("");
    setBusyRowId(null);
    setCurrentPageIndex(0);
    setPageCursors([null]);
    setNextCursor(null);
    setHasMore(false);
  }, [currentUser]);

  useEffect(() => {
    if (!currentUser) {
      return;
    }

    setCurrentPageIndex(0);
    setPageCursors([null]);
    setNextCursor(null);
    setHasMore(false);
  }, [currentUser, activeTab, debouncedSearchTerm]);

  const loadHistoryPage = useCallback(async () => {
    if (!currentUser) {
      return;
    }

    setIsHistoryLoading(true);
    setHistoryError("");

    try {
      const response = await api.historyList({
        limit: PAGE_SIZE,
        cursor: currentCursor || undefined,
        platform: activeTab === "all" ? undefined : activeTab,
        q: debouncedSearchTerm.trim() || undefined,
      });

      const responseItems = response.items || [];
      const responseNextCursor = response.page?.next_cursor || null;

      setHistoryItems(responseItems);
      setHasMore(Boolean(response.page?.has_more));
      setNextCursor(responseNextCursor);

      setPageCursors((previous) => {
        const next = [...previous];
        next[currentPageIndex] = currentCursor;

        if (responseNextCursor) {
          next[currentPageIndex + 1] = responseNextCursor;
        } else if (next.length > currentPageIndex + 1) {
          next.splice(currentPageIndex + 1);
        }

        return next;
      });
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        setCurrentUser(null);
        setHistoryError("Session expired. Please login again.");
      } else {
        setHistoryError(
          error instanceof Error
            ? error.message
            : "Failed to load download history.",
        );
      }

      setHistoryItems([]);
      setHasMore(false);
      setNextCursor(null);
    } finally {
      setIsHistoryLoading(false);
    }
  }, [
    activeTab,
    currentCursor,
    currentPageIndex,
    currentUser,
    debouncedSearchTerm,
    setCurrentUser,
  ]);

  const loadStats = useCallback(async () => {
    if (!currentUser) {
      return;
    }

    setIsStatsLoading(true);

    try {
      const nextStats = await api.historyStats();
      setStats(nextStats);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        setCurrentUser(null);
      }
    } finally {
      setIsStatsLoading(false);
    }
  }, [currentUser, setCurrentUser]);

  useEffect(() => {
    if (!currentUser) {
      return;
    }

    void loadHistoryPage();
  }, [currentUser, loadHistoryPage, refreshTick]);

  useEffect(() => {
    if (!currentUser) {
      return;
    }

    void loadStats();
  }, [currentUser, loadStats, refreshTick]);

  const bumpRefresh = useCallback(() => {
    setRefreshTick((value) => value + 1);
  }, []);

  const pollMp3JobUntilDone = useCallback(async (jobID: string) => {
    const deadline = Date.now() + MP3_JOB_POLL_TIMEOUT_MS;

    while (Date.now() < deadline) {
      const status = await api.getJobStatus(jobID);
      const normalized = (status.status || "").toLowerCase();

      if (normalized === "done") {
        if (!status.download_url) {
          throw new Error("MP3 job finished without download URL.");
        }
        return status.download_url;
      }

      if (normalized === "failed") {
        throw new Error(status.error || "MP3 redownload failed.");
      }

      await sleep(MP3_JOB_POLL_INTERVAL_MS);
    }

    throw new Error("Timed out waiting for MP3 redownload job.");
  }, []);

  const handleDownloadAgain = useCallback(
    async (id: string) => {
      if (busyRowId) {
        return;
      }

      setBusyRowId(id);
      setActionError("");
      setActionMessage("");

      try {
        const response = await api.historyRedownload(id);

        if (response.mode === "direct") {
          if (!response.download_url) {
            throw new Error("Download URL is missing from redownload response.");
          }

          triggerBrowserDownload(response.download_url);
          setActionMessage("Download started.");
          bumpRefresh();
          return;
        }

        if (response.mode === "queued") {
          if (!response.job_id) {
            throw new Error("Redownload job was queued without a job ID.");
          }

          setActionMessage(
            "Redownload queued. We will start download automatically when ready.",
          );

          void pollMp3JobUntilDone(response.job_id)
            .then((downloadUrl) => {
              triggerBrowserDownload(downloadUrl);
              setActionMessage("MP3 is ready. Download started.");
              setActionError("");
              bumpRefresh();
            })
            .catch((error) => {
              setActionMessage("");
              setActionError(
                error instanceof Error
                  ? error.message
                  : "Queued redownload failed.",
              );
            });

          bumpRefresh();
          return;
        }

        throw new Error("Unsupported redownload mode.");
      } catch (error) {
        if (error instanceof APIError && error.code === "invalid_session") {
          setCurrentUser(null);
          setActionError("Session expired. Please login again.");
        } else {
          setActionError(
            error instanceof Error
              ? error.message
              : "Failed to trigger redownload.",
          );
        }
      } finally {
        setBusyRowId(null);
      }
    },
    [busyRowId, bumpRefresh, pollMp3JobUntilDone, setCurrentUser],
  );

  const handleDelete = useCallback(
    async (id: string) => {
      if (busyRowId) {
        return;
      }

      setBusyRowId(id);
      setActionError("");
      setActionMessage("");

      const wasLastItemOnPage = historyItems.length === 1;

      try {
        await api.historyDelete(id);
        setActionMessage("History item deleted.");

        if (wasLastItemOnPage && currentPageIndex > 0) {
          setCurrentPageIndex((value) => Math.max(0, value - 1));
        }

        bumpRefresh();
      } catch (error) {
        if (error instanceof APIError && error.code === "invalid_session") {
          setCurrentUser(null);
          setActionError("Session expired. Please login again.");
        } else {
          setActionError(
            error instanceof Error
              ? error.message
              : "Failed to delete history item.",
          );
        }
      } finally {
        setBusyRowId(null);
      }
    },
    [busyRowId, bumpRefresh, currentPageIndex, historyItems.length, setCurrentUser],
  );

  const handleNextPage = useCallback(() => {
    if (!hasMore || !nextCursor || isHistoryLoading) {
      return;
    }

    setPageCursors((previous) => {
      const next = [...previous];
      next[currentPageIndex + 1] = nextCursor;
      return next;
    });

    setCurrentPageIndex((value) => value + 1);
  }, [currentPageIndex, hasMore, isHistoryLoading, nextCursor]);

  const handlePreviousPage = useCallback(() => {
    if (!hasPrevPage || isHistoryLoading) {
      return;
    }

    setCurrentPageIndex((value) => Math.max(0, value - 1));
  }, [hasPrevPage, isHistoryLoading]);

  const historyTableData = useMemo<HistoryTableItem[]>(() => {
    return historyItems.map((item) => {
      const latest = item.latest_attempt;
      const requestKind: HistoryRequestKind = latest?.request_kind || "mp4";

      return {
        id: item.id,
        title: item.title || "Untitled media",
        quality: latest?.quality_label || fallbackQualityLabel(requestKind),
        platform: item.platform,
        size: formatBytes(latest?.size_bytes),
        date: formatDateLabel(item.last_attempt_at),
        thumbnail: item.thumbnail_url || "",
        status: latest?.status || "done",
        requestKind,
      };
    });
  }, [historyItems]);

  const statsCards = useMemo<HistoryStatCard[]>(() => {
    return [
      {
        icon: "download",
        label: "Total Downloads",
        value:
          isStatsLoading && !stats
            ? "..."
            : `${formatInteger(stats?.total_items ?? 0)} Items`,
      },
      {
        icon: "database",
        label: "Storage Used",
        value:
          isStatsLoading && !stats
            ? "..."
            : formatBytes(stats?.total_bytes_downloaded ?? 0),
      },
      {
        icon: "calendar",
        label: "This Month",
        value:
          isStatsLoading && !stats
            ? "..."
            : `${formatInteger(stats?.this_month_attempts ?? 0)} Attempts`,
      },
    ];
  }, [isStatsLoading, stats]);

  const paginationTotalItems = hasActiveFilters
    ? null
    : stats?.total_items ?? null;

  if (isAuthChecking) {
    return (
      <div className="relative flex min-h-screen w-full flex-col">
        <Navbar />
        <main className="flex flex-1 items-center justify-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary" />
        </main>
        <Footer />
      </div>
    );
  }

  return (
    <div className="relative flex min-h-screen w-full flex-col overflow-x-hidden">
      <div className="layout-container flex h-full grow flex-col">
        <Navbar />

        <main className="flex flex-1 justify-center py-8 px-4 md:px-10">
          <div className="layout-content-container flex flex-col max-w-[1200px] flex-1">
            {!currentUser ? (
              <div className="flex flex-col items-center justify-center py-20 px-4 text-center">
                <div className="w-20 h-20 bg-primary/10 rounded-full flex items-center justify-center text-primary mb-6">
                  <Lock size={40} weight="fill" />
                </div>
                <h2 className="text-2xl font-bold text-slate-900 dark:text-slate-100 mb-2">
                  Login Required
                </h2>
                <p className="text-slate-500 dark:text-slate-400 max-w-md mb-8">
                  You need to be logged in to view your download history and
                  manage your saved media.
                </p>
                <div className="flex gap-4">
                  <button
                    onClick={() => setLoginModalOpen(true)}
                    className="px-8 py-3 bg-primary text-white font-bold rounded-lg shadow-lg shadow-primary/20 hover:brightness-110 transition-all"
                  >
                    Login Now
                  </button>
                  <button
                    onClick={() => router.push("/")}
                    className="px-8 py-3 bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 font-bold rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 transition-all"
                  >
                    Go Home
                  </button>
                </div>
              </div>
            ) : (
              <>
                <div className="flex flex-col md:flex-row justify-between items-start md:items-end gap-4 mb-8">
                  <div className="flex flex-col gap-1">
                    <h1 className="text-slate-900 dark:text-slate-100 text-3xl font-bold leading-tight tracking-tight">
                      Download History
                    </h1>
                    <p className="text-slate-500 dark:text-slate-400 text-base">
                      Manage and re-download your saved media content
                    </p>
                  </div>
                  <div className="flex gap-2">
                    <SearchBar value={searchTerm} onChange={setSearchTerm} />
                  </div>
                </div>

                {historyError ? (
                  <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-950/30 dark:text-red-300">
                    {historyError}
                  </div>
                ) : null}

                {actionError ? (
                  <div className="mb-4 rounded-lg border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700 dark:border-red-900/30 dark:bg-red-950/30 dark:text-red-300">
                    {actionError}
                  </div>
                ) : null}

                {actionMessage ? (
                  <div className="mb-4 rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/30 dark:bg-emerald-950/30 dark:text-emerald-300">
                    {actionMessage}
                  </div>
                ) : null}

                <div className="mb-6">
                  <HistoryTabs activeTab={activeTab} onChange={setActiveTab} />
                </div>

                <HistoryTable
                  data={historyTableData}
                  onDownloadAgain={handleDownloadAgain}
                  onDelete={handleDelete}
                  isLoading={isHistoryLoading}
                  busyRowId={busyRowId}
                />

                <div className="overflow-hidden rounded-xl border border-t-0 border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm">
                  <Pagination
                    currentPage={currentPageIndex + 1}
                    pageSize={PAGE_SIZE}
                    currentCount={historyTableData.length}
                    totalItems={paginationTotalItems}
                    hasNextPage={hasMore}
                    hasPrevPage={hasPrevPage}
                    isLoading={isHistoryLoading}
                    onNext={handleNextPage}
                    onPrevious={handlePreviousPage}
                  />
                </div>

                <StatsCards stats={statsCards} />
              </>
            )}
          </div>
        </main>

        <Footer />
      </div>
    </div>
  );
}

function useDebouncedValue(value: string, delayMs: number): string {
  const [debounced, setDebounced] = useState(value);

  useEffect(() => {
    const timeoutID = window.setTimeout(() => {
      setDebounced(value);
    }, delayMs);

    return () => {
      window.clearTimeout(timeoutID);
    };
  }, [delayMs, value]);

  return debounced;
}

function fallbackQualityLabel(kind: HistoryRequestKind): string {
  switch (kind) {
    case "mp3":
      return "Audio (MP3)";
    case "image":
      return "Image";
    case "mp4":
    default:
      return "Video (MP4)";
  }
}

function formatBytes(value?: number | null): string {
  if (typeof value !== "number" || !Number.isFinite(value) || value < 0) {
    return "—";
  }

  if (value === 0) {
    return "0 B";
  }

  const units = ["B", "KB", "MB", "GB", "TB"];
  let size = value;
  let unitIndex = 0;

  while (size >= 1024 && unitIndex < units.length - 1) {
    size /= 1024;
    unitIndex += 1;
  }

  const decimals = size >= 100 || unitIndex === 0 ? 0 : size >= 10 ? 1 : 2;
  return `${size.toFixed(decimals)} ${units[unitIndex]}`;
}

function formatDateLabel(value: string): string {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "—";
  }

  return date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

function formatInteger(value: number): string {
  if (!Number.isFinite(value)) {
    return "0";
  }

  return new Intl.NumberFormat("en-US").format(Math.max(0, Math.trunc(value)));
}

function normalizeDownloadURL(input: string): string {
  const trimmed = input.trim();
  if (!trimmed) {
    return "";
  }

  if (/^https?:\/\//i.test(trimmed) || trimmed.startsWith("/")) {
    return trimmed;
  }

  return `/${trimmed}`;
}

function triggerBrowserDownload(downloadURL: string): void {
  const href = normalizeDownloadURL(downloadURL);
  if (!href) {
    return;
  }

  const link = document.createElement("a");
  link.href = href;
  link.target = "_blank";
  link.rel = "noopener noreferrer";
  link.setAttribute("download", "");
  document.body.appendChild(link);
  link.click();
  document.body.removeChild(link);
}

function sleep(ms: number): Promise<void> {
  return new Promise((resolve) => {
    window.setTimeout(resolve, ms);
  });
}
