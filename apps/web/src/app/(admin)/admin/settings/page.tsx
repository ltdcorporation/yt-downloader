"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
} from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import SettingsProfile from "@/components/settings/SettingsProfile";
import {
  DEFAULT_AVATAR_URL,
  type UserProfile,
} from "@/data/settings-data";
import {
  Layout,
  Users,
  Gear,
  Wrench,
} from "@phosphor-icons/react";

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
  const [serverSnapshot, setServerSnapshot] = useState<ServerSnapshot | null>(
    null,
  );

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSaving, setIsSaving] = useState(false);
  const [isAvatarMutating, setIsAvatarMutating] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [saveError, setSaveError] = useState("");
  const [saveSuccess, setSaveSuccess] = useState("");

  const isDirty = useMemo(() => {
    if (!serverSnapshot) {
      return false;
    }

    return profileFullName !== serverSnapshot.profile.fullName;
  }, [profileFullName, serverSnapshot]);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        // Session snapshot cleanup
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
      const profileResponse = await api.profile();

      const nextSnapshot: ServerSnapshot = {
        profile: {
          fullName: profileResponse.profile.full_name,
          email: profileResponse.profile.email,
          avatarURL: profileResponse.profile.avatar_url || "",
        },
      };

      setProfileFullName(nextSnapshot.profile.fullName);
      setProfileEmail(nextSnapshot.profile.email);
      setProfileAvatarURL(nextSnapshot.profile.avatarURL);
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
          : "Failed to load profile. Please refresh and try again.",
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

  const handleDiscard = () => {
    if (!serverSnapshot) {
      return;
    }

    setProfileFullName(serverSnapshot.profile.fullName);
    setProfileEmail(serverSnapshot.profile.email);
    setProfileAvatarURL(serverSnapshot.profile.avatarURL);
    setSaveError("");
    setSaveSuccess("");
  };

  const handleSave = async () => {
    if (!serverSnapshot || isSaving) {
      return;
    }

    setIsSaving(true);
    setSaveError("");
    setSaveSuccess("");

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

      const nextSnapshot: ServerSnapshot = {
        profile: {
          fullName: finalFullName,
          email: finalEmail,
          avatarURL: finalAvatarURL,
        },
      };

      setProfileFullName(finalFullName);
      setProfileEmail(finalEmail);
      setProfileAvatarURL(finalAvatarURL);
      setServerSnapshot(nextSnapshot);
      setSaveSuccess("Profile updated successfully.");
    } catch (error) {
      setSaveError(
        error instanceof Error
          ? error.message
          : "Failed to save profile. Please try again.",
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
              Manage your administrator profile and account details.
            </p>
          </div>

          {loadError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {loadError}
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
