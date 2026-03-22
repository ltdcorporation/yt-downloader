"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import Navbar from "@/components/shared/Navbar";
import Footer from "@/components/shared/Footer";
import SearchBar from "@/components/history/SearchBar";
import HistoryTabs from "@/components/history/HistoryTabs";
import HistoryTable from "@/components/history/HistoryTable";
import Pagination from "@/components/history/Pagination";
import StatsCards from "@/components/history/StatsCards";
import { useHistoryFilter } from "@/hooks/useHistoryFilter";
import { SAMPLE_DATA, STATS } from "@/data/history-sample-data";
import { useAuthStore } from "@/store";
import { Lock } from "@phosphor-icons/react";

export default function HistoryPage() {
  const { currentUser, isAuthChecking, setLoginModalOpen } = useAuthStore();
  const router = useRouter();

  const { searchTerm, activeTab, setSearchTerm, setActiveTab, filteredData } =
    useHistoryFilter({ data: currentUser ? SAMPLE_DATA : [] });

  const handleDownloadAgain = (id: string) => {
    console.log("Download again:", id);
  };

  const handleDelete = (id: string) => {
    console.log("Delete:", id);
  };

  if (isAuthChecking) {
    return (
      <div className="relative flex min-h-screen w-full flex-col">
        <Navbar />
        <main className="flex flex-1 items-center justify-center">
          <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
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
                  You need to be logged in to view your download history and manage your saved media.
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
                {/* Page Title Area */}
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

                {/* Tabs */}
                <div className="mb-6">
                  <HistoryTabs
                    activeTab={activeTab}
                    onChange={setActiveTab}
                  />
                </div>

                {/* History Table */}
                <HistoryTable
                  data={filteredData}
                  onDownloadAgain={handleDownloadAgain}
                  onDelete={handleDelete}
                />

                {/* Pagination */}
                <div className="overflow-hidden rounded-xl border border-t-0 border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 shadow-sm">
                  <Pagination
                    currentPage={1}
                    totalPages={3}
                    totalItems={SAMPLE_DATA.length}
                  />
                </div>

                {/* Footer Summary Stats */}
                <StatsCards stats={STATS} />
              </>
            )}
          </div>
        </main>
        <Footer />
      </div>
    </div>
  );
}
