const translations = {
  en: {
    appTitle: "YouTube Downloader Utility",
    subtitle: "Paste a YouTube URL, select an available quality, then download.",
    inputLabel: "YouTube URL",
    inputPlaceholder: "https://www.youtube.com/watch?v=...",
    stepResolve: "Resolve formats",
    resolving: "Resolving...",
    stepDownload: "Download",
    queueing: "Queueing...",
    sectionFormats: "Available Formats",
    noFormats: "No formats yet. Resolve a URL first.",
    note:
      "MP4 is redirect-based. MP3 uses queue processing (128 kbps) and expires in 1 hour.",
    quality: "Quality",
    container: "Container",
    action: "Action",
    duration: "Duration",
    status: "Status",
    jobID: "Job ID",
    mp3StatusTitle: "MP3 Job Status",
    downloadMP3: "Download MP3",
    errURLRequired: "URL is required.",
    errResolveFailed: "Failed to resolve video formats.",
    errQueueFailed: "Failed to create MP3 job.",
    errNetwork: "Network request failed.",
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
    resolving: "Lagi ambil format...",
    stepDownload: "Download",
    queueing: "Lagi antriin...",
    sectionFormats: "Format Tersedia",
    noFormats: "Belum ada format. Resolve URL dulu.",
    note:
      "MP4 berbasis redirect. MP3 diproses via queue (128 kbps) dan expired 1 jam.",
    quality: "Kualitas",
    container: "Kontainer",
    action: "Aksi",
    duration: "Durasi",
    status: "Status",
    jobID: "ID Job",
    mp3StatusTitle: "Status Job MP3",
    downloadMP3: "Download MP3",
    errURLRequired: "URL wajib diisi.",
    errResolveFailed: "Gagal ambil format video.",
    errQueueFailed: "Gagal bikin job MP3.",
    errNetwork: "Request jaringan gagal.",
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
