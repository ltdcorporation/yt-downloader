"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type AdminJobRecord,
  type AdminUsersStats,
  type AuthUser,
  type MaintenanceServiceItem,
} from "@/lib/api";
import { persistAdminAuth } from "@/lib/auth-session";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import {
  Layout,
  Users,
  Wrench,
  Gear,
  User,
  SealCheck,
  DownloadSimple,
  ClockCountdown,
  CheckCircle,
  Warning,
  UserCirclePlus,
  ArrowSquareOut,
  Database,
  Cpu,
  Pulse,
} from "@phosphor-icons/react";

type ActivityItemType = "user" | "job";

interface ActivityItem {
  id: string;
  type: ActivityItemType;
  title: string;
  description: string;
  time: string;
  sortAt: number;
  badge?: string;
}

interface MetricCard {
  id: string;
  title: string;
  value: string;
  subtitle: string;
  icon: React.ElementType;
}

function formatNumber(value: number): string {
  return new Intl.NumberFormat("en-US").format(value);
}

function formatRelativeTime(value: string): string {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "just now";
  }

  const diffMs = Date.now() - parsed.getTime();
  if (diffMs < 60_000) {
    return "just now";
  }
  const minutes = Math.floor(diffMs / 60_000);
  if (minutes < 60) {
    return `${minutes}m ago`;
  }
  const hours = Math.floor(minutes / 60);
  if (hours < 24) {
    return `${hours}h ago`;
  }
  const days = Math.floor(hours / 24);
  return `${days}d ago`;
}

function serviceIcon(name: string): React.ElementType {
  const key = name.toLowerCase();
  if (key.includes("database")) {
    return Database;
  }
  if (key.includes("worker")) {
    return Pulse;
  }
  return Cpu;
}

function serviceStatusBadge(status: string, enabled: boolean): string {
  if (!enabled) {
    return "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
  }
  switch (status) {
    case "active":
      return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/20 dark:text-emerald-400";
    case "maintenance":
      return "bg-amber-100 text-amber-700 dark:bg-amber-900/20 dark:text-amber-400";
    case "scaling":
      return "bg-blue-100 text-blue-700 dark:bg-blue-900/20 dark:text-blue-400";
    default:
      return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
  }
}

function planDistribution(stats: AdminUsersStats) {
  const total = Math.max(stats.total_users, 1);
  return [
    { label: "Monthly", count: stats.monthly_users, color: "bg-violet-500" },
    { label: "Weekly", count: stats.weekly_users, color: "bg-blue-500" },
    { label: "Daily", count: stats.daily_users, color: "bg-amber-500" },
    { label: "Free", count: stats.free_users, color: "bg-slate-400" },
  ].map((item) => ({
    ...item,
    percentage: Math.round((item.count / total) * 100),
  }));
}

export default function AdminDashboardPage() {
  const {
    currentUser,
    logout,
    isAuthChecking,
    setCurrentUser,
    setIsAuthChecking,
  } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [loginForm, setLoginForm] = useState({ user: "", pass: "" });
  const [loginError, setLoginError] = useState("");
  const [isLoggingIn, setIsLoggingIn] = useState(false);
  const [loadError, setLoadError] = useState("");

  const [stats, setStats] = useState<AdminUsersStats | null>(null);
  const [recentUsers, setRecentUsers] = useState<AuthUser[]>([]);
  const [recentJobs, setRecentJobs] = useState<AdminJobRecord[]>([]);
  const [services, setServices] = useState<MaintenanceServiceItem[]>([]);

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

  const loadDashboard = useCallback(async () => {
    setIsPageLoading(true);
    setLoadError("");

    try {
      const dashboard = await api.getAdminDashboard({
        usersLimit: 8,
        jobsLimit: 20,
      });

      setStats(dashboard.stats);
      setRecentUsers(dashboard.users.items);
      setRecentJobs(dashboard.jobs.items);
      setServices(dashboard.maintenance.maintenance.services);
    } catch (error) {
      const message =
        error instanceof APIError ? error.message : "Failed to load dashboard";
      setLoadError(message);
    } finally {
      setIsPageLoading(false);
    }
  }, []);

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
      setShowLoginModal(true);
      setIsPageLoading(false);
      return;
    }

    setShowLoginModal(false);
    void loadDashboard();
  }, [currentUser, isAuthChecking, loadDashboard]);

  const handleAdminLogin = async (e: React.FormEvent) => {
    e.preventDefault();
    setLoginError("");
    setIsLoggingIn(true);

    try {
      const res = await api.loginAdmin(loginForm.user, loginForm.pass);
      const credentialsBase64 = btoa(`${loginForm.user}:${loginForm.pass}`);
      persistAdminAuth(res.user, credentialsBase64);
      setCurrentUser(res.user);
      setShowLoginModal(false);
      setIsPageLoading(true);
    } catch (error) {
      setLoginError(
        error instanceof Error ? error.message : "Invalid credentials",
      );
    } finally {
      setIsLoggingIn(false);
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

  const metrics = useMemo<MetricCard[]>(() => {
    if (!stats) {
      return [];
    }

    const doneJobs = recentJobs.filter((job) => job.status === "done").length;
    const activeJobs = recentJobs.filter(
      (job) => job.status === "queued" || job.status === "processing",
    ).length;

    return [
      {
        id: "total-users",
        title: "Total Users",
        value: formatNumber(stats.total_users),
        subtitle: `${formatNumber(stats.member_users)} members`,
        icon: User,
      },
      {
        id: "paid-users",
        title: "Active Paid",
        value: formatNumber(stats.active_paid_users),
        subtitle: `${formatNumber(stats.monthly_users)} monthly`,
        icon: SealCheck,
      },
      {
        id: "admin-users",
        title: "Admin Accounts",
        value: formatNumber(stats.admin_users),
        subtitle: "Privileged operators",
        icon: Users,
      },
      {
        id: "jobs-overview",
        title: "Recent Jobs",
        value: `${doneJobs}/${recentJobs.length}`,
        subtitle: `${activeJobs} active in queue`,
        icon: DownloadSimple,
      },
    ];
  }, [recentJobs, stats]);

  const activities = useMemo<ActivityItem[]>(() => {
    const userActivities: ActivityItem[] = recentUsers.map((user) => ({
      id: `user-${user.id}`,
      type: "user",
      title: "New user registered",
      description: `${user.full_name} (${user.email})`,
      time: formatRelativeTime(user.created_at),
      sortAt: new Date(user.created_at).getTime() || 0,
      badge: user.plan,
    }));

    const jobActivities: ActivityItem[] = recentJobs.map((job) => ({
      id: `job-${job.id}`,
      type: "job",
      title: `Job ${job.status}`,
      description: job.title || job.input_url || job.id,
      time: formatRelativeTime(job.updated_at),
      sortAt: new Date(job.updated_at).getTime() || 0,
      badge: job.output_kind,
    }));

    return [...userActivities, ...jobActivities]
      .sort((a, b) => b.sortAt - a.sortAt)
      .slice(0, 10);
  }, [recentJobs, recentUsers]);

  const distributions = useMemo(() => {
    if (!stats) {
      return [];
    }
    return planDistribution(stats);
  }, [stats]);

  if (isAuthChecking || (isPageLoading && !showLoginModal)) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (showLoginModal) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-50 p-4 dark:bg-slate-950">
        <div className="w-full max-w-md rounded-2xl border border-slate-200 bg-white p-8 shadow-2xl dark:border-slate-800 dark:bg-slate-900">
          <div className="mb-8 text-center">
            <div className="mb-4 inline-flex size-14 items-center justify-center rounded-xl bg-primary/10 text-primary">
              <SealCheck size={32} weight="fill" />
            </div>
            <h1 className="text-2xl font-black text-slate-900 dark:text-slate-50">
              Admin Access
            </h1>
            <p className="mt-1 text-sm text-slate-500 dark:text-slate-400">
              Please enter your system credentials
            </p>
          </div>

          <form onSubmit={handleAdminLogin} className="space-y-4">
            <div>
              <label className="ml-1 mb-1.5 block text-[10px] font-black uppercase tracking-widest text-slate-400">
                Username
              </label>
              <input
                type="text"
                value={loginForm.user}
                onChange={(e) =>
                  setLoginForm({ ...loginForm, user: e.target.value })
                }
                className="w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 font-medium text-slate-900 outline-none transition-all focus:border-primary focus:ring-2 focus:ring-primary/20 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-100"
                placeholder="admin"
                required
              />
            </div>

            <div>
              <label className="ml-1 mb-1.5 block text-[10px] font-black uppercase tracking-widest text-slate-400">
                Password
              </label>
              <input
                type="password"
                value={loginForm.pass}
                onChange={(e) =>
                  setLoginForm({ ...loginForm, pass: e.target.value })
                }
                className="w-full rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 font-medium text-slate-900 outline-none transition-all focus:border-primary focus:ring-2 focus:ring-primary/20 dark:border-slate-700 dark:bg-slate-800/50 dark:text-slate-100"
                placeholder="••••••••"
                required
              />
            </div>

            {loginError ? (
              <div className="flex items-center gap-3 rounded-lg border border-rose-100 bg-rose-50 p-3 dark:border-rose-800 dark:bg-rose-900/20">
                <Warning size={18} className="shrink-0 text-rose-500" />
                <p className="text-xs font-bold text-rose-600 dark:text-rose-400">
                  {loginError}
                </p>
              </div>
            ) : null}

            <button
              type="submit"
              disabled={isLoggingIn}
              className="mt-2 w-full rounded-xl bg-primary py-3.5 font-black text-white shadow-lg shadow-primary/20 transition-all hover:bg-primary-dark disabled:opacity-70"
            >
              {isLoggingIn ? "Verifying..." : "Grant Access"}
            </button>

            <button
              type="button"
              onClick={() => router.push("/")}
              className="w-full bg-transparent py-2 text-xs font-bold text-slate-400 transition-colors hover:text-slate-600 dark:hover:text-slate-200"
            >
              Back to Homepage
            </button>
          </form>
        </div>
      </div>
    );
  }

  if (!currentUser || currentUser.role !== "admin") {
    return null;
  }

  const userProfile = {
    name: currentUser.full_name,
    email: currentUser.email,
    plan: "Super Admin",
    avatar: currentUser.avatar_url || DEFAULT_AVATAR_URL,
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
          title="Admin Console"
          showText={false}
        />

        <div className="max-w-6xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-8">
          <div className="flex items-end justify-between gap-4">
            <div>
              <h2 className="text-2xl font-black tracking-tight text-slate-900 dark:text-slate-50">
                Dashboard Overview
              </h2>
              <p className="text-slate-500 dark:text-slate-400 text-sm font-medium">
                Live telemetry from users, jobs, and maintenance services.
              </p>
            </div>
            <button
              onClick={() => void loadDashboard()}
              className="px-4 py-2 text-xs font-black bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg hover:bg-slate-50 dark:hover:bg-slate-700"
            >
              Refresh
            </button>
          </div>

          {loadError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {loadError}
            </div>
          ) : null}

          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
            {metrics.map((metric) => (
              <div
                key={metric.id}
                className="bg-white dark:bg-slate-900 p-6 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800"
              >
                <div className="flex justify-between items-start mb-4">
                  <div className="p-2 bg-primary/10 rounded-lg text-primary">
                    <metric.icon size={24} weight="fill" />
                  </div>
                </div>
                <h3 className="text-slate-500 dark:text-slate-400 text-[11px] font-bold uppercase tracking-wider mb-1">
                  {metric.title}
                </h3>
                <p className="text-2xl font-black text-slate-900 dark:text-slate-50 tabular-nums">
                  {metric.value}
                </p>
                <p className="text-[10px] font-bold text-slate-400 uppercase mt-2 tracking-widest">
                  {metric.subtitle}
                </p>
              </div>
            ))}
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            <div className="lg:col-span-2 space-y-6">
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
                <div className="px-6 py-5 border-b border-slate-100 dark:border-slate-800 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/30">
                  <h3 className="font-bold text-slate-900 dark:text-slate-50">
                    Live Activity Feed
                  </h3>
                  <button
                    onClick={() => router.push("/admin/users")}
                    className="text-xs font-bold text-primary hover:text-primary/80 transition-colors inline-flex items-center gap-1"
                  >
                    Open users
                    <ArrowSquareOut size={14} />
                  </button>
                </div>

                <div className="divide-y divide-slate-100 dark:divide-slate-800">
                  {activities.length === 0 ? (
                    <div className="p-6 text-sm text-slate-500 dark:text-slate-400">
                      No recent activity.
                    </div>
                  ) : (
                    activities.map((item) => (
                      <div
                        key={item.id}
                        className="p-4 flex items-center gap-4 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-all"
                      >
                        <div className="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center flex-shrink-0">
                          {item.type === "user" ? (
                            <UserCirclePlus size={20} className="text-blue-500" />
                          ) : (
                            <DownloadSimple size={20} className="text-emerald-500" />
                          )}
                        </div>
                        <div className="flex-1 min-w-0">
                          <div className="flex items-center gap-2">
                            <p className="text-sm font-bold text-slate-900 dark:text-slate-50 truncate">
                              {item.title}
                            </p>
                            {item.badge ? (
                              <span className="text-[9px] font-black px-1.5 py-0.5 rounded uppercase tracking-tighter bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300">
                                {item.badge}
                              </span>
                            ) : null}
                          </div>
                          <p className="text-xs text-slate-500 dark:text-slate-400 truncate">
                            {item.description}
                          </p>
                        </div>
                        <p className="text-[10px] font-bold text-slate-400 uppercase whitespace-nowrap">
                          {item.time}
                        </p>
                      </div>
                    ))
                  )}
                </div>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <h3 className="font-bold text-slate-900 dark:text-slate-50 mb-4">
                  Recent Jobs
                </h3>
                <div className="space-y-3">
                  {recentJobs.slice(0, 6).map((job) => (
                    <div
                      key={job.id}
                      className="flex items-center justify-between p-3 rounded-lg bg-slate-50 dark:bg-slate-800/40 border border-slate-100 dark:border-slate-800"
                    >
                      <div className="min-w-0">
                        <p className="text-sm font-semibold text-slate-900 dark:text-slate-100 truncate">
                          {job.title || job.input_url || job.id}
                        </p>
                        <p className="text-[11px] text-slate-500 dark:text-slate-400 truncate">
                          {job.id}
                        </p>
                      </div>
                      <div className="flex items-center gap-3 ml-4">
                        <span className="text-[10px] font-black uppercase text-slate-500 dark:text-slate-400">
                          {job.status}
                        </span>
                        <span className="text-[10px] font-bold text-slate-400 whitespace-nowrap inline-flex items-center gap-1">
                          <ClockCountdown size={12} />
                          {formatRelativeTime(job.updated_at)}
                        </span>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <div className="space-y-6">
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <h3 className="font-bold text-slate-900 dark:text-slate-50 mb-6">
                  Plan Distribution
                </h3>
                <div className="space-y-4">
                  {distributions.map((item) => (
                    <div key={item.label}>
                      <div className="flex justify-between items-center mb-1">
                        <div className="flex items-center gap-2">
                          <div className={`size-2.5 rounded-sm ${item.color}`} />
                          <span className="text-xs font-bold text-slate-600 dark:text-slate-400">
                            {item.label}
                          </span>
                        </div>
                        <span className="text-xs font-black text-slate-900 dark:text-slate-50 tabular-nums">
                          {item.count} ({item.percentage}%)
                        </span>
                      </div>
                      <div className="w-full bg-slate-100 dark:bg-slate-800 h-2 rounded-full overflow-hidden">
                        <div
                          className={`${item.color} h-full rounded-full`}
                          style={{ width: `${item.percentage}%` }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <div className="flex items-center justify-between mb-4">
                  <h3 className="font-bold text-slate-900 dark:text-slate-50">
                    System Health
                  </h3>
                  <button
                    onClick={() => router.push("/admin/maintenance")}
                    className="text-xs font-bold text-primary"
                  >
                    Open
                  </button>
                </div>
                <div className="space-y-3">
                  {services.map((service) => {
                    const Icon = serviceIcon(service.name);
                    return (
                      <div
                        key={service.key}
                        className="flex items-center justify-between p-3 rounded-lg bg-slate-50 dark:bg-slate-800/40 border border-slate-100 dark:border-slate-800"
                      >
                        <div className="flex items-center gap-2 min-w-0">
                          <Icon size={16} className="text-slate-500" />
                          <span className="text-xs font-semibold text-slate-700 dark:text-slate-300 truncate">
                            {service.name}
                          </span>
                        </div>
                        <span
                          className={`text-[10px] font-black uppercase px-2 py-0.5 rounded ${serviceStatusBadge(
                            service.status,
                            service.enabled,
                          )}`}
                        >
                          {service.enabled ? service.status : "disabled"}
                        </span>
                      </div>
                    );
                  })}
                </div>
              </div>

              <div className="bg-slate-900 dark:bg-slate-800 rounded-xl shadow-xl p-6 text-white border border-slate-800">
                <div className="flex items-center gap-2 mb-3">
                  <CheckCircle size={18} className="text-emerald-400" />
                  <h4 className="font-bold text-sm">Admin Operations</h4>
                </div>
                <p className="text-slate-400 text-xs leading-relaxed mb-4">
                  Use the Users and Maintenance tabs for direct operational
                  changes. This dashboard is read-focused telemetry.
                </p>
                <div className="flex gap-2">
                  <button
                    onClick={() => router.push("/admin/users")}
                    className="flex-1 bg-white text-slate-900 hover:bg-slate-100 py-2 rounded-lg text-xs font-black"
                  >
                    Users
                  </button>
                  <button
                    onClick={() => router.push("/admin/maintenance")}
                    className="flex-1 bg-white/10 hover:bg-white/20 text-white py-2 rounded-lg text-xs font-black"
                  >
                    Maintenance
                  </button>
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
