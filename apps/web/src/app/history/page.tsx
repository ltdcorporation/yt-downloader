"use client";

import Navbar from "@/components/shared/Navbar";
import Footer from "@/components/shared/Footer";
import SearchBar from "@/components/history/SearchBar";
import HistoryTabs from "@/components/history/HistoryTabs";
import HistoryTable from "@/components/history/HistoryTable";
import Pagination from "@/components/history/Pagination";
import StatsCards from "@/components/history/StatsCards";
import { useHistoryFilter } from "@/hooks/useHistoryFilter";
import { SAMPLE_DATA, STATS } from "@/data/history-sample-data";

export default function HistoryPage() {
  const { searchTerm, activeTab, setSearchTerm, setActiveTab, filteredData } =
    useHistoryFilter({ data: SAMPLE_DATA });

  const handleDownloadAgain = (id: string) => {
    console.log("Download again:", id);
  };

  const handleDelete = (id: string) => {
    console.log("Delete:", id);
  };

  return (
    <div className="relative flex min-h-screen w-full flex-col overflow-x-hidden">
      <div className="layout-container flex h-full grow flex-col">
        <Navbar />
        <main className="flex flex-1 justify-center py-8 px-4 md:px-10">
          <div className="layout-content-container flex flex-col max-w-[1200px] flex-1">
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
          </div>
        </main>
        <Footer />
      </div>
    </div>
  );
}
