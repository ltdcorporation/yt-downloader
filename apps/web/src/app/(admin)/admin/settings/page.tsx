"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type AdminSettingsPatchRequest,
  type AdminSettingsSnapshotResponse,
} from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import SettingsProfile from "@/components/settings/SettingsProfile";
import SettingsPreferences from "@/components/settings/SettingsPreferences";
import SettingsNotifications from "@/components/settings/SettingsNotifications";
import {
  DEFAULT_AVATAR_URL,
  buildDefaultSettingsFormData,
  fromNotificationPreferences,
  toNotificationPreferences,
  type EmailAlertSettings,
  type SettingsFormData,
  type UserProfile,
} from "@/data/settings-data";
import { Layout, Users, Gear, Wrench } from "@phosphor-icons/react";

const AVATAR_MAX_BYTES = 2 * 1024 * 1024;
const ALLOWED_AVATAR_TYPES = new Set([
  "image/jpeg",
  "image/jpg",
  "image/png",
  "image/gif",
  "image/webp",
]);

interface AdminSettingsSnapshotState {
  version: number;
  updatedAt: string;
  updatedByUserID: string;
  defaultQuality: SettingsFormData["defaultQuality"];
  autoTrimSilence: boolean;
  thumbnailGeneration: boolean;
  emailAlerts: EmailAlertSettings;
}

interface ServerSnapshot {
  profile: {
    fullName: string;
    email: string;
    avatarURL: string;
  };
  settings: AdminSettingsSnapshotState;
}

function mapAdminSettingsSnapshot(
  snapshot: AdminSettingsSnapshotResponse,
): AdminSettingsSnapshotState {
  return {
    version: snapshot.meta.version,
    updatedAt: snapshot.meta.updated_at,
    updatedByUserID: snapshot.meta.updated_by_user_id || "",
    defaultQuality: snapshot.settings.preferences.default_quality,
    autoTrimSilence: snapshot.settings.preferences.auto_trim_silence,
    thumbnailGeneration: snapshot.settings.preferences.thumbnail_generation,
    emailAlerts: {
      processing: snapshot.settings.notifications.email.processing,
      storage: snapshot.settings.notifications.email.storage,
      summary: snapshot.settings.notifications.email.summary,
    },
  };
}

function mapSnapshotSettingsToForm(
  snapshot: AdminSettingsSnapshotState,
): SettingsFormData {
  return {
    defaultQuality: snapshot.defaultQuality,
    autoTrimSilence: snapshot.autoTrimSilence,
    thumbnailGeneration: snapshot.thumbnailGeneration,
    emailAlerts: toNotificationPreferences(snapshot.emailAlerts),
  };
}

function buildAdminSettingsPatch(
  baseline: AdminSettingsSnapshotState,
  formData: SettingsFormData,
): AdminSettingsPatchRequest["settings"] | null {
  const patch: AdminSettingsPatchRequest["settings"] = {};

  const preferences: NonNullable<AdminSettingsPatchRequest["settings"]["preferences"]> =
    {};

  if (formData.defaultQuality !== baseline.defaultQuality) {
    preferences.default_quality = formData.defaultQuality;
  }
  if (formData.autoTrimSilence !== baseline.autoTrimSilence) {
    preferences.auto_trim_silence = formData.autoTrimSilence;
  }
  if (formData.thumbnailGeneration !== baseline.thumbnailGeneration) {
    preferences.thumbnail_generation = formData.thumbnailGeneration;
  }

  if (Object.keys(preferences).length > 0) {
    patch.preferences = preferences;
  }

  const nextEmailAlerts = fromNotificationPreferences(formData.emailAlerts);
  const notificationsEmail: NonNullable<
    NonNullable<AdminSettingsPatchRequest["settings"]["notifications"]>["email"]
  > = {};

  if (nextEmailAlerts.processing !== baseline.emailAlerts.processing) {
    notificationsEmail.processing = nextEmailAlerts.processing;
  }
  if (nextEmailAlerts.storage !== baseline.emailAlerts.storage) {
    notificationsEmail.storage = nextEmailAlerts.storage;
  }
  if (nextEmailAlerts.summary !== baseline.emailAlerts.summary) {
    notificationsEmail.summary = nextEmailAlerts.summary;
  }

  if (Object.keys(notificationsEmail).length > 0) {
    patch.notifications = {
      email: notificationsEmail,
    };
  }

  if (Object.keys(patch).length === 0) {
    return null;
  }

  return patch;
}

function formatUpdatedAt(value: string): string {
  if (!value) {
    return "";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }

  return parsed.toLocaleString("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export default function AdminSettingsPage() {
  const {
    currentUser,
    logout,
    isAuthChecking,
    setCurrentUser,
    setIsAuthChecking,
  } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();

  const [profileFullName, setProfileFullName] = useState("");
  const [profileEmail, setProfileEmail] = useState("");
  const [profileAvatarURL, setProfileAvatarURL] = useState("");
  const [formData, setFormData] = useState<SettingsFormData>(
    buildDefaultSettingsFormData(),
  );
  const [serverSnapshot, setServerSnapshot] = useState<ServerSnapshot | null>(
    null,
  );

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isAvatarMutating, setIsAvatarMutating] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [saveError, setSaveError] = useState("");
  const [saveSuccess, setSaveSuccess] = useState("");
  const [conflictNotice, setConflictNotice] = useState("");

  const isDirty = useMemo(() => {
    if (!serverSnapshot) {
      return false;
    }

    const settingsPatch = buildAdminSettingsPatch(serverSnapshot.settings, formData);

    return (
      profileFullName !== serverSnapshot.profile.fullName ||
      settingsPatch !== null
    );
  }, [formData, profileFullName, serverSnapshot]);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        // session invalid; clear auth snapshot below
      }
      setCurrentUser(null);
    } finally {
      setIsAuthChecking(false);
    }
  }, [setCurrentUser, setIsAuthChecking]);

  useEffect(() => {
    if (isAuthChecking) {
      void refreshAuthState();
    }
  }, [isAuthChecking, refreshAuthState]);

  const loadSettings = useCallback(async () => {
    if (!currentUser) {
      return;
    }

    setIsPageLoading(true);
    setLoadError("");

    try {
      const [profileResponse, adminSettingsResponse] = await Promise.all([
        api.profile(),
        api.getAdminSettings(),
      ]);

      const settingsSnapshot = mapAdminSettingsSnapshot(adminSettingsResponse);
      const nextSnapshot: ServerSnapshot = {
        profile: {
          fullName: profileResponse.profile.full_name,
          email: profileResponse.profile.email,
          avatarURL: profileResponse.profile.avatar_url || "",
        },
        settings: settingsSnapshot,
      };

      setProfileFullName(nextSnapshot.profile.fullName);
      setProfileEmail(nextSnapshot.profile.email);
      setProfileAvatarURL(nextSnapshot.profile.avatarURL);
      setFormData(mapSnapshotSettingsToForm(settingsSnapshot));
      setServerSnapshot(nextSnapshot);
    } catch (error) {
      if (
        error instanceof APIError &&
        (error.code === "invalid_session" || error.code === "session_expired")
      ) {
        setCurrentUser(null);
        router.push("/");
        return;
      }

      setLoadError(
        error instanceof Error
          ? error.message
          : "Failed to load settings. Please refresh and try again.",
      );
    } finally {
      setIsPageLoading(false);
    }
  }, [currentUser, router, setCurrentUser]);

  useEffect(() => {
    if (isAuthChecking) {
      return;
    }

    if (!currentUser || currentUser.role !== "admin") {
      router.push("/");
      return;
    }

    void loadSettings();
  }, [currentUser, isAuthChecking, loadSettings, router]);

  useEffect(() => {
    if (!isDirty) {
      return;
    }

    const handleBeforeUnload = (event: BeforeUnloadEvent) => {
      event.preventDefault();
      event.returnValue = "";
    };

    window.addEventListener("beforeunload", handleBeforeUnload);
    return () => {
      window.removeEventListener("beforeunload", handleBeforeUnload);
    };
  }, [isDirty]);

  const handleQualityChange = (value: SettingsFormData["defaultQuality"]) => {
    setFormData((prev) => ({ ...prev, defaultQuality: value }));
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");
  };

  const handleAutoTrimChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, autoTrimSilence: value }));
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");
  };

  const handleThumbnailChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, thumbnailGeneration: value }));
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");
  };

  const handleAlertChange = (id: string, checked: boolean) => {
    setFormData((prev) => ({
      ...prev,
      emailAlerts: prev.emailAlerts.map((alert) =>
        alert.id === id ? { ...alert, checked } : alert,
      ),
    }));
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");
  };

  const handleDiscard = () => {
    if (!serverSnapshot) {
      return;
    }

    setProfileFullName(serverSnapshot.profile.fullName);
    setProfileEmail(serverSnapshot.profile.email);
    setProfileAvatarURL(serverSnapshot.profile.avatarURL);
    setFormData(mapSnapshotSettingsToForm(serverSnapshot.settings));
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");
  };

  const handleSave = async () => {
    if (!serverSnapshot || isSaving) {
      return;
    }

    setIsSaving(true);
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");

    try {
      let finalProfile = { ...serverSnapshot.profile };

      if (profileFullName !== serverSnapshot.profile.fullName) {
        const profileResponse = await api.updateProfile({
          fullName: profileFullName,
        });
        finalProfile = {
          fullName: profileResponse.profile.full_name,
          email: profileResponse.profile.email,
          avatarURL: profileResponse.profile.avatar_url || "",
        };
        setCurrentUser(profileResponse.profile);
      }

      const settingsPatch = buildAdminSettingsPatch(serverSnapshot.settings, formData);
      let nextSettingsSnapshot = serverSnapshot.settings;

      if (settingsPatch) {
        const settingsResponse = await api.updateAdminSettings({
          settings: settingsPatch,
          meta: {
            version: serverSnapshot.settings.version,
          },
        });
        nextSettingsSnapshot = mapAdminSettingsSnapshot(settingsResponse);
      }

      const nextSnapshot: ServerSnapshot = {
        profile: finalProfile,
        settings: nextSettingsSnapshot,
      };

      setProfileFullName(finalProfile.fullName);
      setProfileEmail(finalProfile.email);
      setProfileAvatarURL(finalProfile.avatarURL);
      setFormData(mapSnapshotSettingsToForm(nextSettingsSnapshot));
      setServerSnapshot(nextSnapshot);
      setSaveSuccess("Admin settings updated successfully.");
    } catch (error) {
      if (
        error instanceof APIError &&
        error.code === "admin_settings_version_conflict"
      ) {
        try {
          const latest = await api.getAdminSettings();
          const latestSnapshot = mapAdminSettingsSnapshot(latest);
          setServerSnapshot((prev) =>
            prev
              ? {
                  ...prev,
                  settings: latestSnapshot,
                }
              : prev,
          );
          setFormData(mapSnapshotSettingsToForm(latestSnapshot));
          setConflictNotice(
            "Settings were changed elsewhere. Latest version has been loaded.",
          );
        } catch {
          setConflictNotice(
            "Settings changed elsewhere. Please refresh and try again.",
          );
        }
      } else {
        setSaveError(
          error instanceof Error
            ? error.message
            : "Failed to save settings. Please try again.",
        );
      }
    } finally {
      setIsSaving(false);
    }
  };

  const handleAvatarUpload = async (file: File) => {
    if (!serverSnapshot || isAvatarMutating) {
      return;
    }

    if (file.type && !ALLOWED_AVATAR_TYPES.has(file.type.toLowerCase())) {
      setSaveError("Unsupported image format. Use JPG, PNG, GIF, or WEBP.");
      return;
    }

    if (file.size > AVATAR_MAX_BYTES) {
      setSaveError("Profile photo exceeds 2MB. Please choose a smaller image.");
      return;
    }

    setIsAvatarMutating(true);
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");

    try {
      const profileResponse = await api.uploadProfileAvatar(file);
      const nextAvatarURL = profileResponse.profile.avatar_url || "";

      setProfileAvatarURL(nextAvatarURL);
      setCurrentUser(profileResponse.profile);
      setServerSnapshot((prev) =>
        prev
          ? {
              ...prev,
              profile: {
                ...prev.profile,
                avatarURL: nextAvatarURL,
                fullName: profileResponse.profile.full_name || prev.profile.fullName,
                email: profileResponse.profile.email || prev.profile.email,
              },
            }
          : prev,
      );
      setSaveSuccess("Profile photo updated.");
    } catch (error) {
      setSaveError(
        error instanceof Error
          ? error.message
          : "Failed to upload profile photo. Please try again.",
      );
    } finally {
      setIsAvatarMutating(false);
    }
  };

  const handleAvatarRemove = async () => {
    if (!serverSnapshot || isAvatarMutating) {
      return;
    }

    setIsAvatarMutating(true);
    setSaveError("");
    setSaveSuccess("");
    setConflictNotice("");

    try {
      const profileResponse = await api.removeProfileAvatar();
      const nextAvatarURL = profileResponse.profile.avatar_url || "";

      setProfileAvatarURL(nextAvatarURL);
      setCurrentUser(profileResponse.profile);
      setServerSnapshot((prev) =>
        prev
          ? {
              ...prev,
              profile: {
                ...prev.profile,
                avatarURL: nextAvatarURL,
                fullName: profileResponse.profile.full_name || prev.profile.fullName,
                email: profileResponse.profile.email || prev.profile.email,
              },
            }
          : prev,
      );
      setSaveSuccess("Profile photo removed.");
    } catch (error) {
      setSaveError(
        error instanceof Error
          ? error.message
          : "Failed to remove profile photo. Please try again.",
      );
    } finally {
      setIsAvatarMutating(false);
    }
  };

  const [isSidebarOpen, setIsSidebarOpen] = useState(false);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  if (isAuthChecking || isPageLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!currentUser) {
    return null;
  }

  const effectiveUserName = profileFullName || currentUser.full_name;
  const effectiveUserEmail = profileEmail || currentUser.email;

  const userProfile: UserProfile = {
    name: effectiveUserName,
    email: effectiveUserEmail,
    plan: "Super Admin",
    avatar: profileAvatarURL || currentUser.avatar_url || DEFAULT_AVATAR_URL,
  };

  const adminNavItems = [
    {
      icon: Layout,
      label: "Dashboard",
      href: "/admin",
      active: pathname === "/admin",
    },
    {
      icon: Users,
      label: "Users",
      href: "/admin/users",
      active: pathname.startsWith("/admin/users"),
    },
    {
      icon: Wrench,
      label: "Maintenance",
      href: "/admin/maintenance",
      active: pathname === "/admin/maintenance",
    },
    {
      icon: Gear,
      label: "Settings",
      href: "/admin/settings",
      active: pathname === "/admin/settings",
    },
  ];

  return (
    <div className="flex h-screen overflow-hidden">
      <SettingsSidebar
        user={userProfile}
        onLogout={handleLogout}
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
        navItems={adminNavItems}
      />
      <main className="flex-1 overflow-y-auto bg-background-light dark:bg-background-dark">
        <SettingsHeader
          onMenuClick={() => setIsSidebarOpen(true)}
          title="Admin Settings"
          showText={false}
        />
        <div className="max-w-4xl mx-auto px-8 pb-12 space-y-6">
          <div className="pt-4">
            <h2 className="text-2xl font-black text-slate-900 dark:text-slate-50 tracking-tight">
              Admin Settings
            </h2>
            <p className="text-slate-500 dark:text-slate-400 text-sm mt-1 font-medium">
              Manage administrator profile and global application defaults.
            </p>
            {serverSnapshot ? (
              <p className="text-xs text-slate-400 dark:text-slate-500 mt-2">
                Settings version {serverSnapshot.settings.version} · last updated{" "}
                {formatUpdatedAt(serverSnapshot.settings.updatedAt) || "just now"}
                {serverSnapshot.settings.updatedByUserID
                  ? ` by ${serverSnapshot.settings.updatedByUserID}`
                  : ""}
              </p>
            ) : null}
          </div>

          {loadError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {loadError}
            </div>
          ) : null}

          {conflictNotice ? (
            <div className="rounded-lg border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-800 dark:border-amber-900/60 dark:bg-amber-950/30 dark:text-amber-300">
              {conflictNotice}
            </div>
          ) : null}

          {saveError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {saveError}
            </div>
          ) : null}

          {saveSuccess ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300">
              {saveSuccess}
            </div>
          ) : null}

          <SettingsProfile
            user={userProfile}
            fullName={profileFullName}
            email={profileEmail}
            onFullNameChange={setProfileFullName}
            onAvatarUpload={handleAvatarUpload}
            onAvatarRemove={handleAvatarRemove}
            avatarBusy={isAvatarMutating}
            emailReadOnly
          />

          <SettingsPreferences
            defaultQuality={formData.defaultQuality}
            autoTrimSilence={formData.autoTrimSilence}
            thumbnailGeneration={formData.thumbnailGeneration}
            onQualityChange={handleQualityChange}
            onAutoTrimChange={handleAutoTrimChange}
            onThumbnailChange={handleThumbnailChange}
          />

          <SettingsNotifications
            emailAlerts={formData.emailAlerts}
            onAlertChange={handleAlertChange}
          />

          <div className="flex items-center justify-end gap-3 pt-4">
            <button
              onClick={handleDiscard}
              disabled={!isDirty || isSaving || isAvatarMutating}
              className="px-6 py-2.5 text-sm font-bold text-slate-600 dark:text-slate-400 hover:text-slate-900 disabled:cursor-not-allowed disabled:opacity-50 dark:hover:text-slate-100 transition-colors"
            >
              Discard Changes
            </button>
            <button
              onClick={handleSave}
              disabled={!isDirty || isSaving || isAvatarMutating}
              className="px-8 py-2.5 bg-primary text-white text-sm font-bold rounded-lg shadow-lg shadow-primary/20 hover:bg-primary/90 disabled:cursor-not-allowed disabled:opacity-60 transition-all"
            >
              {isSaving ? "Saving..." : "Save Changes"}
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
