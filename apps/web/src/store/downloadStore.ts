import { create } from "zustand";

interface DownloadState {
  url: string;
  setUrl: (url: string) => void;
  resetUrl: () => void;
}

export const useDownloadStore = create<DownloadState>((set) => ({
  url: "",
  setUrl: (url) => set({ url }),
  resetUrl: () => set({ url: "" }),
}));
