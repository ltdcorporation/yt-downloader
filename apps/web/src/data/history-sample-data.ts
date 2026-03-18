export interface DownloadHistory {
  id: string;
  title: string;
  quality: string;
  platform: "youtube" | "tiktok" | "instagram";
  size: string;
  date: string;
  thumbnail: string;
}

export interface StatCard {
  icon: "download" | "database" | "calendar";
  label: string;
  value: string;
}

export const SAMPLE_DATA: DownloadHistory[] = [
  {
    id: "1",
    title: "Pro Pasta Cooking Masterclass 2024",
    quality: "High Quality (1080p)",
    platform: "youtube",
    size: "142.5 MB",
    date: "Oct 24, 2023",
    thumbnail: "https://lh3.googleusercontent.com/aida-public/AB6AXuAWOjcV4UR-6mLFicIwP5m2MFjMbcpysMp-sTO7fUpvUZrGnqAXA2UVeMjFlhhJP3ottsNixzB_nt5HU3KUwjbTtel6BVBe0PMMD-e2yaZHBaDKTbpf5EUh1l1RD1fznPGlgnJgTYJhX9X1nH1kZlHaQ6AbqaxHyaqehiBIgO-lBf7OrhM5oragiFDM623FsnN2-3p34xAmMygDehwQMtl7sG-Pv2ellwKhRqXSm8mxgsWoyImuStmfMxX3MRLG2L7O7xre",
  },
  {
    id: "2",
    title: "Amazing Street Performance Viral #shorts",
    quality: "Mobile Optimized",
    platform: "tiktok",
    size: "18.2 MB",
    date: "Oct 22, 2023",
    thumbnail: "https://lh3.googleusercontent.com/aida-public/AB6AXuDX_9O_7jIG1-3XqUZO-nAVVgRLzHeUEwmr127EcNIYqi_lOZ3MSrRsyptAoEYbWMjCNWkkDHTvWNi284OPpUcvk3DrUY5vrnaPD06WJb27UJIZezTZF_LZr5udZnN5UD785hWpkLjUcPYqhSsg0yda2riVdCOs6mYgd_lpunJNrTaoDO_omcoPyN5W5jlBrJsHnqtAxrrxb2r9d0cW3_oRVUmOQBqK-gOJuqUzvnJyST52RBGdmPhL5-o9P6RWCySZFlX3xjOt",
  },
  {
    id: "3",
    title: "How to Build a Custom Desk Setup",
    quality: "Full HD (1080p)",
    platform: "instagram",
    size: "54.0 MB",
    date: "Oct 20, 2023",
    thumbnail: "https://lh3.googleusercontent.com/aida-public/AB6AXuCb4YthMR6a2g0fEGkVAKGbI5XPzys6aAoSoQVlACcBCxn6yhvEwyXIXe7EgtaVmOB74rhGNgEghtQ7S93xqjLhSlfsml_Y_5uV877HJPdja-DAXZ08hrD2Cv9DfF3ziNWwvZSPkNmLWMohzWcBmwT-nYc3RyCSCL1PHFFI9GS1CPARhcwgEIXibdWMXXn_kCk2nzggao09o1TZDX6GgWTAff_S7lALK22oql9FKN6lDU9w3KCtOJIaRM4GQc0qNmxXQ0nCE3SM",
  },
  {
    id: "4",
    title: "Top 10 Gaming Moments of the Week",
    quality: "Standard Quality (720p)",
    platform: "youtube",
    size: "320.1 MB",
    date: "Oct 19, 2023",
    thumbnail: "https://lh3.googleusercontent.com/aida-public/AB6AXuBAKt-8kc7cdCELjQq6JFG2BbsxJNms_su83yivk4ztca0cQ-Mx8kL6y1kQmo3xxDD1uzBRq-9VOPX_vGmIslRTFa0iwcdOz39k8_PH6lnCXVVei_UiWnJd5Uvv_5g5pSbmqPUaa54gbLwGYcQRFTXlCHGR_hsf_FCB6MkRTaGZl2qv5l8Js8Qz6ZqraergzjoPFOPSuKL2rHXDfir0WkRnFncIHsQEiipCwFms9g4r9Ph13tigdrQwgZBnRPgrG6oL0BZh907N",
  },
];

export const STATS: StatCard[] = [
  { icon: "download", label: "Total Downloads", value: "158 Items" },
  { icon: "database", label: "Storage Used", value: "12.4 GB" },
  { icon: "calendar", label: "This Month", value: "42 Videos" },
];

export const PLATFORM_COLORS: Record<DownloadHistory["platform"], string> = {
  youtube: "bg-red-100 text-red-600",
  tiktok: "bg-slate-900 text-white",
  instagram: "bg-pink-100 text-pink-600",
};
