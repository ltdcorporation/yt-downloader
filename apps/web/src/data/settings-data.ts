export interface UserProfile {
  name: string;
  email: string;
  avatar: string;
  plan: string;
}

export interface NotificationPreference {
  id: string;
  label: string;
  checked: boolean;
}

export interface SettingsFormData {
  fullName: string;
  email: string;
  defaultQuality: "4k" | "1080p" | "720p" | "480p";
  autoTrimSilence: boolean;
  thumbnailGeneration: boolean;
  emailAlerts: NotificationPreference[];
}

export const SAMPLE_USER: UserProfile = {
  name: "Alex Johnson",
  email: "alex.johnson@example.com",
  avatar:
    "https://lh3.googleusercontent.com/aida-public/AB6AXuANDr4_pSbk5fjDd0HDrsZfvTjJZFjhn7FCp25AvDMb0TVC-6H5afnfzzYbSxpbcIJ7zoyvevJR2yAqyZUeeBE2e6ZjC2sugHkEvQCj2EBXoblXae1C1PeFlX2S_Vb5fjHY8oW3g_QkBJNlfLqjl_jPcHbhY6m2uIzle82n5OwsDEPV0jr1cdq1SJ4a4G-DT8j8YNlJpevBUBuVKfWm5d_Q4tmGPpDnt1baTHduUY4ynVRR4OG3YlEWIqzNtMLthmVoSEkfILJ1",
  plan: "Pro Plan",
};

export const DEFAULT_SETTINGS: SettingsFormData = {
  fullName: "Alex Johnson",
  email: "alex.johnson@example.com",
  defaultQuality: "1080p",
  autoTrimSilence: true,
  thumbnailGeneration: false,
  emailAlerts: [
    { id: "processing", label: "Processing completed successfully", checked: true },
    { id: "storage", label: "Low storage warnings", checked: true },
    { id: "summary", label: "Monthly summary report", checked: false },
  ],
};

export const QUALITY_OPTIONS = [
  { value: "4k", label: "4K (Ultra HD)" },
  { value: "1080p", label: "1080p (Full HD)" },
  { value: "720p", label: "720p (HD)" },
  { value: "480p", label: "480p (Standard)" },
] as const;
