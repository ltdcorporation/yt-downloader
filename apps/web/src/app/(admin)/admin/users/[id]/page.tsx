"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, usePathname, useParams } from "next/navigation";
import Image from "next/image";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type AuthUser,
  type UserPlan,
  type UserRole,
} from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import {
  Layout,
  Users,
  Gear,
  Wrench,
  ArrowLeft,
  Calendar,
  Crown,
  User as UserIcon,
  FloppyDisk,
  ArrowClockwise,
  Clock,
  CheckCircle,
} from "@phosphor-icons/react";

interface UserFormState {
  fullName: string;
  role: UserRole;
  plan: UserPlan;
  planExpiresAt: string;
}

const PLAN_OPTIONS: Array<{ value: UserPlan; label: string }> = [
  { value: "free", label: "Free" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
];

function toLocalInputValue(value?: string): string {
  if (!value) {
    return "";
  }
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }

  const year = parsed.getFullYear();
  const month = String(parsed.getMonth() + 1).padStart(2, "0");
  const day = String(parsed.getDate()).padStart(2, "0");
  const hours = String(parsed.getHours()).padStart(2, "0");
  const minutes = String(parsed.getMinutes()).padStart(2, "0");
  return `${year}-${month}-${day}T${hours}:${minutes}`;
}

function normalizeLocalDateTime(value: string): string {
  if (!value) {
    return "";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "";
  }

  return parsed.toISOString();
}

function formatDate(value?: string): string {
  if (!value) {
    return "-";
  }

  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "-";
  }

  return parsed.toLocaleDateString("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  });
}

function planBadgeStyles(plan: UserPlan): string {
  switch (plan) {
    case "monthly":
      return "bg-violet-100 text-violet-700 dark:bg-violet-900/30 dark:text-violet-400";
    case "weekly":
      return "bg-blue-100 text-blue-700 dark:bg-blue-900/30 dark:text-blue-400";
    case "daily":
      return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
    default:
      return "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
  }
}

function roleBadgeStyles(role: UserRole): string {
  return role === "admin"
    ? "bg-primary/10 text-primary"
    : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
}

function buildForm(user: AuthUser): UserFormState {
  return {
    fullName: user.full_name,
    role: user.role,
    plan: user.plan,
    planExpiresAt: toLocalInputValue(user.plan_expires_at),
  };
}

export default function AdminUserDetailPage() {
  const {
    currentUser,
    logout,
    isAuthChecking,
    setCurrentUser,
    setIsAuthChecking,
  } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const userId = decodeURIComponent(params.id as string);

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [user, setUser] = useState<AuthUser | null>(null);
  const [form, setForm] = useState<UserFormState | null>(null);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch {
      setCurrentUser(null);
    } finally {
      setIsAuthChecking(false);
    }
  }, [setCurrentUser, setIsAuthChecking]);

  const loadUser = useCallback(async () => {
    setIsPageLoading(true);
    setLoadError("");

    try {
      const fetched = await api.getAdminUser(userId);
      setUser(fetched);
      setForm(buildForm(fetched));
    } catch (error) {
      const message =
        error instanceof APIError ? error.message : "Failed to load user";
      setLoadError(message);
    } finally {
      setIsPageLoading(false);
    }
  }, [userId]);

  useEffect(() => {
    if (isAuthChecking) {
      void refreshAuthState();
    }
  }, [isAuthChecking, refreshAuthState]);

  useEffect(() => {
    if (isAuthChecking) {
      return;
    }
    if (!currentUser || currentUser.role !== "admin") {
      router.push("/");
      return;
    }

    void loadUser();
  }, [currentUser, isAuthChecking, loadUser, router]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  const isDirty = useMemo(() => {
    if (!user || !form) {
      return false;
    }

    return (
      form.fullName.trim() !== user.full_name ||
      form.role !== user.role ||
      form.plan !== user.plan ||
      form.planExpiresAt !== toLocalInputValue(user.plan_expires_at)
    );
  }, [form, user]);

  const handleDiscard = () => {
    if (!user) {
      return;
    }
    setForm(buildForm(user));
    setActionMessage("Changes discarded.");
    setLoadError("");
  };

  const handleSave = async () => {
    if (!user || !form || !isDirty) {
      return;
    }

    const payload: {
      full_name?: string;
      role?: UserRole;
      plan?: UserPlan;
      plan_expires_at?: string;
    } = {};

    if (form.fullName.trim() !== user.full_name) {
      payload.full_name = form.fullName.trim();
    }
    if (form.role !== user.role) {
      payload.role = form.role;
    }
    if (form.plan !== user.plan) {
      payload.plan = form.plan;
    }

    const initialPlanExpiresAt = toLocalInputValue(user.plan_expires_at);
    if (form.planExpiresAt !== initialPlanExpiresAt) {
      payload.plan_expires_at = form.planExpiresAt
        ? normalizeLocalDateTime(form.planExpiresAt)
        : "";
    }

    setIsSaving(true);
    setLoadError("");
    setActionMessage("");

    try {
      const updated = await api.updateAdminUser(user.id, payload);
      setUser(updated);
      setForm(buildForm(updated));
      setActionMessage("User profile updated.");

      if (currentUser?.id === updated.id) {
        setCurrentUser({
          ...currentUser,
          role: updated.role,
          plan: updated.plan,
          plan_expires_at: updated.plan_expires_at,
          full_name: updated.full_name,
        });
      }
    } catch (error) {
      const message =
        error instanceof APIError ? error.message : "Failed to update user";
      setLoadError(message);
    } finally {
      setIsSaving(false);
    }
  };

  if (isAuthChecking || isPageLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!currentUser || !user || !form) {
    return null;
  }

  const userProfile = {
    name: currentUser.full_name,
    email: currentUser.email,
    plan: "Super Admin",
    avatar: currentUser.avatar_url || DEFAULT_AVATAR_URL,
  };

  const adminNavItems = [
    { icon: Layout, label: "Dashboard", href: "/admin", active: pathname === "/admin" },
    {
      icon: Users,
      label: "Users",
      href: "/admin/users",
      active: pathname.startsWith("/admin/users"),
    },
    { icon: Wrench, label: "Maintenance", href: "/admin/maintenance", active: pathname === "/admin/maintenance" },
    { icon: Gear, label: "Settings", href: "/admin/settings", active: pathname === "/admin/settings" },
  ];

  return (
    <div className="flex h-screen overflow-hidden bg-background-light dark:bg-background-dark">
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
          title="User Details"
          showText={false}
        />

        <div className="max-w-5xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-6">
          <button
            onClick={() => router.push("/admin/users")}
            className="flex items-center gap-2 text-slate-500 hover:text-primary transition-colors text-sm font-bold group"
          >
            <ArrowLeft size={18} className="group-hover:-translate-x-1 transition-transform" />
            Back to User List
          </button>

          {loadError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {loadError}
            </div>
          ) : null}

          {actionMessage ? (
            <div className="rounded-lg border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-700 dark:border-emerald-900/60 dark:bg-emerald-950/30 dark:text-emerald-300">
              {actionMessage}
            </div>
          ) : null}

          <section className="bg-white dark:bg-slate-900 rounded-xl p-8 border border-slate-200 dark:border-slate-800 shadow-sm">
            <div className="flex flex-col md:flex-row gap-8 items-start">
              <div className="relative">
                <Image
                  src={user.avatar_url || DEFAULT_AVATAR_URL}
                  alt={user.full_name}
                  width={128}
                  height={128}
                  className="size-32 rounded-2xl object-cover border-4 border-slate-50 dark:border-slate-800 shadow-xl"
                />
              </div>

              <div className="flex-1 space-y-4">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div>
                    <h1 className="text-3xl font-black text-slate-900 dark:text-white tracking-tight">
                      {user.full_name}
                    </h1>
                    <p className="text-slate-500 font-medium">{user.email}</p>
                  </div>
                  <div className="flex items-center gap-2">
                    <span
                      className={`inline-flex items-center gap-1 text-xs font-black uppercase px-2 py-1 rounded ${roleBadgeStyles(
                        user.role,
                      )}`}
                    >
                      {user.role === "admin" ? (
                        <Crown size={14} weight="fill" />
                      ) : (
                        <UserIcon size={14} weight="fill" />
                      )}
                      {user.role}
                    </span>
                    <span
                      className={`inline-flex items-center text-xs font-black uppercase px-2 py-1 rounded ${planBadgeStyles(
                        user.plan,
                      )}`}
                    >
                      {user.plan}
                    </span>
                  </div>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t border-slate-100 dark:border-slate-800">
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">
                      Joined
                    </p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">
                      {formatDate(user.created_at)}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">
                      Plan Expires
                    </p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">
                      {formatDate(user.plan_expires_at)}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">
                      User ID
                    </p>
                    <p className="text-xs font-bold text-slate-900 dark:text-white break-all">
                      {user.id}
                    </p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">
                      Access
                    </p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">
                      {user.role === "admin" ? "Administrator" : "Standard User"}
                    </p>
                  </div>
                </div>
              </div>
            </div>
          </section>

          <section className="bg-white dark:bg-slate-900 rounded-xl p-6 border border-slate-200 dark:border-slate-800 shadow-sm space-y-6">
            <h3 className="font-bold text-sm uppercase tracking-widest text-slate-400">
              Edit User Access
            </h3>

            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-500 uppercase tracking-wider">
                  Full Name
                </label>
                <input
                  value={form.fullName}
                  onChange={(event) =>
                    setForm((prev) =>
                      prev ? { ...prev, fullName: event.target.value } : prev,
                    )
                  }
                  className="w-full px-4 py-2.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm font-medium text-slate-900 dark:text-slate-100"
                />
              </div>

              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-500 uppercase tracking-wider">
                  Role
                </label>
                <select
                  value={form.role}
                  onChange={(event) =>
                    setForm((prev) =>
                      prev
                        ? { ...prev, role: event.target.value as UserRole }
                        : prev,
                    )
                  }
                  className="w-full px-4 py-2.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm font-medium text-slate-900 dark:text-slate-100"
                >
                  <option value="user">user</option>
                  <option value="admin">admin</option>
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-500 uppercase tracking-wider">
                  Plan
                </label>
                <select
                  value={form.plan}
                  onChange={(event) =>
                    setForm((prev) =>
                      prev
                        ? { ...prev, plan: event.target.value as UserPlan }
                        : prev,
                    )
                  }
                  className="w-full px-4 py-2.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm font-medium text-slate-900 dark:text-slate-100"
                >
                  {PLAN_OPTIONS.map((option) => (
                    <option key={option.value} value={option.value}>
                      {option.label}
                    </option>
                  ))}
                </select>
              </div>

              <div className="space-y-2">
                <label className="text-xs font-bold text-slate-500 uppercase tracking-wider">
                  Plan Expires At
                </label>
                <input
                  type="datetime-local"
                  value={form.planExpiresAt}
                  onChange={(event) =>
                    setForm((prev) =>
                      prev
                        ? { ...prev, planExpiresAt: event.target.value }
                        : prev,
                    )
                  }
                  className="w-full px-4 py-2.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 text-sm font-medium text-slate-900 dark:text-slate-100"
                />
              </div>
            </div>

            <div className="rounded-lg border border-slate-200 dark:border-slate-700 bg-slate-50 dark:bg-slate-800/40 p-4 text-xs text-slate-600 dark:text-slate-300 leading-relaxed flex items-start gap-2">
              <Clock size={16} className="mt-0.5 text-primary" />
              <p>
                If plan expiry is empty and plan is paid, backend will auto-assign
                default duration.
              </p>
            </div>

            <div className="flex flex-col sm:flex-row gap-3 justify-end">
              <button
                onClick={handleDiscard}
                disabled={!isDirty || isSaving}
                className="px-5 py-2.5 text-xs font-black uppercase tracking-wider text-slate-500 hover:text-slate-900 dark:hover:text-white disabled:opacity-40 disabled:cursor-not-allowed"
              >
                <span className="inline-flex items-center gap-2">
                  <ArrowClockwise size={14} />
                  Discard
                </span>
              </button>
              <button
                onClick={() => void handleSave()}
                disabled={!isDirty || isSaving}
                className="px-6 py-2.5 bg-primary text-white text-xs font-black rounded-lg shadow-lg shadow-primary/20 hover:brightness-110 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
              >
                <span className="inline-flex items-center gap-2">
                  {isSaving ? (
                    <div className="size-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                  ) : (
                    <FloppyDisk size={14} weight="fill" />
                  )}
                  {isSaving ? "Saving..." : "Save Changes"}
                </span>
              </button>
            </div>
          </section>

          <section className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-xl border border-slate-800">
            <h4 className="font-bold text-sm mb-2 flex items-center gap-2 text-primary-light">
              <CheckCircle size={18} />
              Access Safety
            </h4>
            <p className="text-slate-400 text-xs leading-relaxed">
              Role and plan updates are validated server-side with audit fields and
              self-demotion protection for admins.
            </p>
          </section>
        </div>
      </main>
    </div>
  );
}
