export interface UserProfile {
  name: string;
  email: string;
  avatar: string;
  plan: string;
}

export type DefaultQuality = "4k" | "1080p" | "720p" | "480p";
export type NotificationPreferenceID = "processing" | "storage" | "summary";

export interface NotificationPreference {
  id: NotificationPreferenceID;
  label: string;
  checked: boolean;
}

export interface SettingsFormData {
  defaultQuality: DefaultQuality;
  autoTrimSilence: boolean;
  thumbnailGeneration: boolean;
  emailAlerts: NotificationPreference[];
}

export interface EmailAlertSettings {
  processing: boolean;
  storage: boolean;
  summary: boolean;
}

export const DEFAULT_AVATAR_URL = "/images/avatar-default.svg";

export const ALERT_LABELS: Record<NotificationPreferenceID, string> = {
  processing: "Processing completed successfully",
  storage: "Low storage warnings",
  summary: "Monthly summary report",
};

export const QUALITY_OPTIONS = [
  { value: "4k", label: "4K (Ultra HD)" },
  { value: "1080p", label: "1080p (Full HD)" },
  { value: "720p", label: "720p (HD)" },
  { value: "480p", label: "480p (Standard)" },
] as const;

export function toNotificationPreferences(
  emailAlerts: EmailAlertSettings,
): NotificationPreference[] {
  return [
    {
      id: "processing",
      label: ALERT_LABELS.processing,
      checked: emailAlerts.processing,
    },
    {
      id: "storage",
      label: ALERT_LABELS.storage,
      checked: emailAlerts.storage,
    },
    {
      id: "summary",
      label: ALERT_LABELS.summary,
      checked: emailAlerts.summary,
    },
  ];
}

export function fromNotificationPreferences(
  alerts: NotificationPreference[],
): EmailAlertSettings {
  const byID = alerts.reduce<Record<NotificationPreferenceID, boolean>>(
    (acc, alert) => {
      acc[alert.id] = alert.checked;
      return acc;
    },
    {
      processing: true,
      storage: true,
      summary: false,
    },
  );

  return {
    processing: byID.processing,
    storage: byID.storage,
    summary: byID.summary,
  };
}

export function buildDefaultSettingsFormData(): SettingsFormData {
  return {
    defaultQuality: "1080p",
    autoTrimSilence: false,
    thumbnailGeneration: false,
    emailAlerts: toNotificationPreferences({
      processing: true,
      storage: true,
      summary: false,
    }),
  };
}
