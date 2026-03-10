const translations = {
  en: {
    appTitle: "YouTube Downloader Utility",
    subtitle: "Paste a YouTube URL, select an available quality, then download.",
    inputLabel: "YouTube URL",
    inputPlaceholder: "https://www.youtube.com/watch?v=...",
    stepResolve: "Resolve formats",
    stepDownload: "Download",
    sectionFormats: "Available Formats",
    noFormats: "No formats yet. Resolve a URL first.",
    note:
      "MP4 is redirect-based. MP3 uses queue processing (128 kbps) and expires in 1 hour.",
    quality: "Quality",
    container: "Container",
    action: "Action",
    adminTitle: "Admin (View Only)",
    adminSubtitle: "Recent jobs and error snapshots."
  },
  id: {
    appTitle: "Utility Downloader YouTube",
    subtitle:
      "Tempel URL YouTube, pilih kualitas yang tersedia, lalu download.",
    inputLabel: "URL YouTube",
    inputPlaceholder: "https://www.youtube.com/watch?v=...",
    stepResolve: "Ambil format",
    stepDownload: "Download",
    sectionFormats: "Format Tersedia",
    noFormats: "Belum ada format. Resolve URL dulu.",
    note:
      "MP4 berbasis redirect. MP3 diproses via queue (128 kbps) dan expired 1 jam.",
    quality: "Kualitas",
    container: "Kontainer",
    action: "Aksi",
    adminTitle: "Admin (Lihat Saja)",
    adminSubtitle: "Snapshot job terbaru dan error."
  }
};

export function getLocale(langValue) {
  return langValue === "id" ? "id" : "en";
}

export function t(locale) {
  return translations[locale] || translations.en;
}
