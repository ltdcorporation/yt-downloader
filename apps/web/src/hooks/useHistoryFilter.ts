import { useState, useMemo } from "react";
import type { DownloadHistory } from "@/data/history-sample-data";

type Platform = "all" | "youtube" | "tiktok" | "instagram";

interface UseHistoryFilterOptions {
  data: DownloadHistory[];
}

interface UseHistoryFilterReturn {
  searchTerm: string;
  activeTab: Platform;
  setSearchTerm: (term: string) => void;
  setActiveTab: (platform: Platform) => void;
  filteredData: DownloadHistory[];
}

export function useHistoryFilter({
  data,
}: UseHistoryFilterOptions): UseHistoryFilterReturn {
  const [searchTerm, setSearchTerm] = useState("");
  const [activeTab, setActiveTab] = useState<Platform>("all");

  const filteredData = useMemo(() => {
    return data.filter((item) => {
      const matchesSearch = item.title
        .toLowerCase()
        .includes(searchTerm.toLowerCase());
      const matchesTab = activeTab === "all" || item.platform === activeTab;
      return matchesSearch && matchesTab;
    });
  }, [data, searchTerm, activeTab]);

  return {
    searchTerm,
    activeTab,
    setSearchTerm,
    setActiveTab,
    filteredData,
  };
}
