"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type SettingsPatchRequest,
  type SettingsSnapshotResponse,
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
  type SettingsFormData,
  type UserProfile,
} from "@/data/settings-data";

const AVATAR_MAX_BYTES = 2 * 1024 * 1024;
const ALLOWED_AVATAR_TYPES = new Set([
  "image/jpeg",
  "image/jpg",
  "image/png",
  "image/gif",
  "image/webp",
]);

interface ServerSnapshot {
  profile: {
    fullName: string;
    email: string;
    avatarURL: string;
  };
  settings: SettingsFormData;
  version: number;
  updatedAt: string;
}

function cloneFormData(input: SettingsFormData): SettingsFormData {
  return {
    ...input,
    emailAlerts: input.emailAlerts.map((alert) => ({ ...alert })),
  };
}

function mapSettingsToForm(response: SettingsSnapshotResponse): SettingsFormData {
  return {
    defaultQuality: response.settings.preferences.default_quality,
    autoTrimSilence: response.settings.preferences.auto_trim_silence,
    thumbnailGeneration: response.settings.preferences.thumbnail_generation,
    emailAlerts: toNotificationPreferences(response.settings.notifications.email),
  };
}

function buildSettingsPatchPayload(
  draft: SettingsFormData,
  baseline: SettingsFormData,
  version: number,
): SettingsPatchRequest | null {
  const preferencesPatch: {
    default_quality?: SettingsFormData["defaultQuality"];
    auto_trim_silence?: boolean;
    thumbnail_generation?: boolean;
  } = {};

  if (draft.defaultQuality !== baseline.defaultQuality) {
    preferencesPatch.default_quality = draft.defaultQuality;
  }
  if (draft.autoTrimSilence !== baseline.autoTrimSilence) {
    preferencesPatch.auto_trim_silence = draft.autoTrimSilence;
  }
  if (draft.thumbnailGeneration !== baseline.thumbnailGeneration) {
    preferencesPatch.thumbnail_generation = draft.thumbnailGeneration;
  }

  const draftAlerts = fromNotificationPreferences(draft.emailAlerts);
  const baselineAlerts = fromNotificationPreferences(baseline.emailAlerts);
  const emailNotificationsPatch: {
    processing?: boolean;
    storage?: boolean;
    summary?: boolean;
  } = {};

  if (draftAlerts.processing !== baselineAlerts.processing) {
    emailNotificationsPatch.processing = draftAlerts.processing;
  }
  if (draftAlerts.storage !== baselineAlerts.storage) {
    emailNotificationsPatch.storage = draftAlerts.storage;
  }
  if (draftAlerts.summary !== baselineAlerts.summary) {
    emailNotificationsPatch.summary = draftAlerts.summary;
  }

  const settingsPatch: SettingsPatchRequest["settings"] = {};
  if (Object.keys(preferencesPatch).length > 0) {
    settingsPatch.preferences = preferencesPatch;
  }
  if (Object.keys(emailNotificationsPatch).length > 0) {
    settingsPatch.notifications = {
      email: emailNotificationsPatch,
    };
  }

  if (Object.keys(settingsPatch).length === 0) {
    return null;
  }

  return {
    settings: settingsPatch,
    meta: {
      version,
    },
  };
}

export default function SettingsPage() {
  const {
    currentUser,
    logout,
    isAuthChecking,
    setCurrentUser,
    setIsAuthChecking,
  } = useAuthStore();
  const router = useRouter();

  const [profileFullName, setProfileFullName] = useState("");
  const [profileEmail, setProfileEmail] = useState("");
  const [profileAvatarURL, setProfileAvatarURL] = useState("");
  const [formData, setFormData] = useState<SettingsFormData>(
    buildDefaultSettingsFormData(),
  );
  const [settingsVersion, setSettingsVersion] = useState<number>(1);
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

    if (profileFullName !== serverSnapshot.profile.fullName) {
      return true;
    }

    const draft = JSON.stringify(formData);
    const baseline = JSON.stringify(serverSnapshot.settings);
    return draft !== baseline;
  }, [formData, profileFullName, serverSnapshot]);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        // Session snapshot cleanup happens in navbar flow; keep store state aligned.
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
      const [profileResponse, settingsResponse] = await Promise.all([
        api.profile(),
        api.getSettings(),
      ]);

      const nextFormData = mapSettingsToForm(settingsResponse);
      const nextSnapshot: ServerSnapshot = {
        profile: {
          fullName: profileResponse.profile.full_name,
          email: profileResponse.profile.email,
          avatarURL: profileResponse.profile.avatar_url || "",
        },
        settings: cloneFormData(nextFormData),
        version: settingsResponse.meta.version,
        updatedAt: settingsResponse.meta.updated_at,
      };

      setProfileFullName(nextSnapshot.profile.fullName);
      setProfileEmail(nextSnapshot.profile.email);
      setProfileAvatarURL(nextSnapshot.profile.avatarURL);
      setFormData(nextFormData);
      setSettingsVersion(nextSnapshot.version);
      setServerSnapshot(nextSnapshot);
      setConflictNotice("");
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
    if (!isAuthChecking && !currentUser) {
      router.push("/");
      return;
    }

    if (currentUser) {
      void loadSettings();
    }
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
  };

  const handleAutoTrimChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, autoTrimSilence: value }));
  };

  const handleThumbnailChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, thumbnailGeneration: value }));
  };

  const handleAlertChange = (id: string, checked: boolean) => {
    setFormData((prev) => ({
      ...prev,
      emailAlerts: prev.emailAlerts.map((alert) =>
        alert.id === id ? { ...alert, checked } : alert,
      ),
    }));
  };

  const handleDiscard = () => {
    if (!serverSnapshot) {
      return;
    }

    setProfileFullName(serverSnapshot.profile.fullName);
    setProfileEmail(serverSnapshot.profile.email);
    setProfileAvatarURL(serverSnapshot.profile.avatarURL);
    setFormData(cloneFormData(serverSnapshot.settings));
    setSettingsVersion(serverSnapshot.version);
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
      let finalFullName = profileFullName;
      let finalEmail = profileEmail;
      let finalAvatarURL = profileAvatarURL;

      if (profileFullName !== serverSnapshot.profile.fullName) {
        const profileResponse = await api.updateProfile({
          fullName: profileFullName,
        });
        finalFullName = profileResponse.profile.full_name;
        finalEmail = profileResponse.profile.email;
        finalAvatarURL = profileResponse.profile.avatar_url || "";
        setCurrentUser(profileResponse.profile);
      }

      const settingsPatch = buildSettingsPatchPayload(
        formData,
        serverSnapshot.settings,
        settingsVersion,
      );

      let finalSettings = cloneFormData(formData);
      let finalVersion = settingsVersion;
      let finalUpdatedAt = serverSnapshot.updatedAt;

      if (settingsPatch) {
        const settingsResponse = await api.updateSettings(settingsPatch);
        finalSettings = mapSettingsToForm(settingsResponse);
        finalVersion = settingsResponse.meta.version;
        finalUpdatedAt = settingsResponse.meta.updated_at;
      }

      const nextSnapshot: ServerSnapshot = {
        profile: {
          fullName: finalFullName,
          email: finalEmail,
          avatarURL: finalAvatarURL,
        },
        settings: cloneFormData(finalSettings),
        version: finalVersion,
        updatedAt: finalUpdatedAt,
      };

      setProfileFullName(finalFullName);
      setProfileEmail(finalEmail);
      setProfileAvatarURL(finalAvatarURL);
      setFormData(finalSettings);
      setSettingsVersion(finalVersion);
      setServerSnapshot(nextSnapshot);
      setSaveSuccess("Settings saved successfully.");
    } catch (error) {
      if (error instanceof APIError && error.code === "settings_version_conflict") {
        setConflictNotice(
          "Your settings were updated from another session. Reload latest data, review changes, then save again.",
        );
      }
      setSaveError(
        error instanceof Error
          ? error.message
          : "Failed to save settings. Please try again.",
      );
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
    plan: "Free Plan",
    avatar: profileAvatarURL || currentUser.avatar_url || DEFAULT_AVATAR_URL,
  };

  return (
    <div className="flex h-screen overflow-hidden">
      <SettingsSidebar user={userProfile} onLogout={handleLogout} />
      <main className="flex-1 overflow-y-auto bg-background-light dark:bg-background-dark">
        <SettingsHeader />
        <div className="max-w-4xl px-8 pb-12 space-y-6">
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
              {isSaving ? "Saving..." : "Save Preferences"}
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
