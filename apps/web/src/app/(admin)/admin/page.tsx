"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError } from "@/lib/api";
import { persistAdminAuth } from "@/lib/auth-session";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import {
  Layout,
  User,
  SealCheck,
  DownloadSimple,
  CurrencyDollar,
  UserCirclePlus,
  Warning,
  CloudArrowDown,
  EnvelopeSimple,
  CheckCircle,
  Users,
  Wrench,
  Gear,
} from "@phosphor-icons/react";

interface MetricCard {
  id: string;
  title: string;
  value: string;
  trend: string;
  trendDirection: "up" | "down" | "stable";
  icon: React.ElementType;
}

interface ActivityItem {
  id: string;
  type: "user" | "alert" | "system" | "marketing";
  title: string;
  description: string;
  time: string;
  badge?: string;
}

const MOCK_METRICS: MetricCard[] = [
  {
    id: "1",
    title: "Total Users",
    value: "842,931",
    trend: "+12.5%",
    trendDirection: "up",
    icon: User,
  },
  {
    id: "2",
    title: "Active Pro",
    value: "124,502",
    trend: "+3.2%",
    trendDirection: "up",
    icon: SealCheck,
  },
  {
    id: "3",
    title: "Downloads Today",
    value: "18,294",
    trend: "Stable",
    trendDirection: "stable",
    icon: DownloadSimple,
  },
  {
    id: "4",
    title: "Daily Revenue",
    value: "$42,930",
    trend: "+18.7%",
    trendDirection: "up",
    icon: CurrencyDollar,
  },
];

const MOCK_ACTIVITIES: ActivityItem[] = [
  {
    id: "1",
    type: "user",
    title: "New Pro User registration",
    description: "marcus.reid@example.com joined the platform",
    time: "2m ago",
    badge: "Pro",
  },
  {
    id: "2",
    type: "alert",
    title: "Server Latency Spike",
    description: "Region US-East experienced 200ms delay",
    time: "14m ago",
    badge: "Alert",
  },
  {
    id: "3",
    type: "system",
    title: "Daily Backup Completed",
    description: "2.4TB of user assets secured",
    time: "1h ago",
    badge: "System",
  },
  {
    id: "4",
    type: "marketing",
    title: "Bulk Newsletter Sent",
    description: "Delivered to 840k active subscribers",
    time: "3h ago",
    badge: "Marketing",
  },
];

export default function AdminDashboardPage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [showLoginModal, setShowLoginModal] = useState(false);
  const [loginForm, setLoginForm] = useState({ user: "", pass: "" });
  const [loginError, setLoginError] = useState("");
  const [isLoggingIn, setIsLoggingIn] = useState(false);

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

  useEffect(() => {
    if (!isAuthChecking) {
      if (!currentUser || currentUser.role !== "admin") {
        setShowLoginModal(true);
        setIsPageLoading(false);
      } else {
        setShowLoginModal(false);
        setIsPageLoading(false);
      }
    }
  }, [currentUser, isAuthChecking]);

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
      setIsPageLoading(false);
    } catch (error) {
      setLoginError(error instanceof Error ? error.message : "Invalid credentials");
    } finally {
      setIsLoggingIn(false);
    }
  };

  const pathname = usePathname();

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  const handleViewAllActivity = () => {
    console.log("View all activity");
  };

  const handleContactSupport = () => {
    console.log("Contact support");
  };

  if (isAuthChecking || (isPageLoading && !showLoginModal)) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (showLoginModal) {
    return (
      <div className="flex h-screen items-center justify-center bg-slate-50 dark:bg-slate-950 p-4">
        <div className="w-full max-w-md bg-white dark:bg-slate-900 rounded-2xl shadow-2xl border border-slate-200 dark:border-slate-800 p-8">
          <div className="mb-8 text-center">
            <div className="inline-flex items-center justify-center size-14 rounded-xl bg-primary/10 text-primary mb-4">
              <SealCheck size={32} weight="fill" />
            </div>
            <h1 className="text-2xl font-black text-slate-900 dark:text-slate-50">Admin Access</h1>
            <p className="text-slate-500 dark:text-slate-400 text-sm mt-1">Please enter your system credentials</p>
          </div>

          <form onSubmit={handleAdminLogin} className="space-y-4">
            <div>
              <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-1.5 ml-1">Username</label>
              <input
                type="text"
                value={loginForm.user}
                onChange={(e) => setLoginForm({ ...loginForm, user: e.target.value })}
                className="w-full px-4 py-3 bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-xl focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all text-slate-900 dark:text-slate-100 font-medium"
                placeholder="admin"
                required
              />
            </div>
            <div>
              <label className="block text-[10px] font-black uppercase tracking-widest text-slate-400 mb-1.5 ml-1">Password</label>
              <input
                type="password"
                value={loginForm.pass}
                onChange={(e) => setLoginForm({ ...loginForm, pass: e.target.value })}
                className="w-full px-4 py-3 bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-xl focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all text-slate-900 dark:text-slate-100 font-medium"
                placeholder="••••••••"
                required
              />
            </div>

            {loginError && (
              <div className="p-3 bg-rose-50 dark:bg-rose-900/20 border border-rose-100 dark:border-rose-800 rounded-lg flex items-center gap-3 animate-shake">
                <Warning size={18} className="text-rose-500 flex-shrink-0" />
                <p className="text-xs font-bold text-rose-600 dark:text-rose-400">{loginError}</p>
              </div>
            )}

            <button
              type="submit"
              disabled={isLoggingIn}
              className="w-full bg-primary hover:bg-primary-dark text-white font-black py-3.5 rounded-xl shadow-lg shadow-primary/20 transition-all active:scale-[0.98] disabled:opacity-70 disabled:active:scale-100 mt-2"
            >
              {isLoggingIn ? "Verifying..." : "Grant Access"}
            </button>
            
            <button
              type="button"
              onClick={() => router.push("/")}
              className="w-full bg-transparent text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 font-bold py-2 text-xs transition-colors"
            >
              Back to Homepage
            </button>
          </form>
        </div>
      </div>
    );
  }

  if (!currentUser) {
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
      active: pathname === "/admin/users",
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

  const getBadgeStyles = (type: ActivityItem["type"]) => {
    switch (type) {
      case "user":
        return "bg-primary/10 text-primary";
      case "alert":
        return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400";
      case "system":
        return "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-400";
      case "marketing":
        return "bg-primary/10 text-primary";
      default:
        return "bg-slate-100 text-slate-600";
    }
  };

  const getTrendBadgeStyles = (direction: MetricCard["trendDirection"]) => {
    switch (direction) {
      case "up":
        return "text-emerald-500 bg-emerald-50 dark:bg-emerald-950/30 dark:text-emerald-400";
      case "down":
        return "text-rose-500 bg-rose-50 dark:bg-rose-950/30 dark:text-rose-400";
      case "stable":
        return "text-slate-400 bg-slate-50 dark:bg-slate-800 dark:text-slate-400";
    }
  };

  return (
    <div className="flex h-screen overflow-hidden bg-background-light dark:bg-background-dark">
      <SettingsSidebar
        user={userProfile}
        onLogout={handleLogout}
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
        navItems={adminNavItems}
      />

      {/* Main Content */}
      <main className="flex-1 overflow-y-auto bg-background-light dark:bg-background-dark">
        <SettingsHeader
          onMenuClick={() => setIsSidebarOpen(true)}
          title="Admin Console"
          showText={false}
        />

        {/* Content Canvas */}
        <div className="max-w-6xl mx-auto pt-4 px-4 sm:px-8 pb-12">
          {/* Header Section */}
          <div className="mb-8">
            <h2 className="text-2xl font-black tracking-tight text-slate-900 dark:text-slate-50">
              Dashboard Overview
            </h2>
            <p className="text-slate-500 dark:text-slate-400 text-sm font-medium">
              Real-time performance metrics for QuickSnap platform.
            </p>
          </div>

          {/* Bento Grid Metrics */}
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6 mb-8">
            {MOCK_METRICS.map((metric) => (
              <div
                key={metric.id}
                className="bg-white dark:bg-slate-900 p-6 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 transition-all hover:shadow-md group"
              >
                <div className="flex justify-between items-start mb-4">
                  <div className="p-2 bg-primary/10 rounded-lg text-primary group-hover:bg-primary group-hover:text-white transition-colors">
                    <metric.icon size={24} weight="fill" />
                  </div>
                  <div className="flex flex-col items-end">
                    <span
                      className={`text-[10px] font-black px-2 py-0.5 rounded-full ${getTrendBadgeStyles(
                        metric.trendDirection
                      )}`}
                    >
                      {metric.trend}
                    </span>
                    <span className="text-[9px] text-slate-400 font-bold mt-1 uppercase">vs last week</span>
                  </div>
                </div>
                <div className="space-y-1">
                  <h3 className="text-slate-500 dark:text-slate-400 text-[11px] font-bold uppercase tracking-wider">
                    {metric.title}
                  </h3>
                  <div className="flex items-end justify-between">
                    <p className="text-2xl font-black text-slate-900 dark:text-slate-50 tabular-nums">
                      {metric.value}
                    </p>
                    {/* Simulated Sparkline */}
                    <div className="flex items-end gap-0.5 h-8 mb-1">
                      {[40, 70, 45, 90, 65, 80, 50].map((h, i) => (
                        <div 
                          key={i} 
                          className="w-1 bg-primary/20 rounded-full" 
                          style={{ height: `${h}%` }}
                        />
                      ))}
                    </div>
                  </div>
                </div>
              </div>
            ))}
          </div>

          {/* Main Dashboard Layout */}
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            {/* Activity Feed (Left 2/3) */}
            <div className="lg:col-span-2 space-y-6">
              {/* Recent Activity */}
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
                <div className="px-6 py-5 border-b border-slate-100 dark:border-slate-800 flex justify-between items-center bg-slate-50/50 dark:bg-slate-800/30">
                  <div className="flex items-center gap-2">
                    <div className="size-2 rounded-full bg-emerald-500 animate-pulse" />
                    <h3 className="font-bold text-slate-900 dark:text-slate-50">
                      Live Activity Feed
                    </h3>
                  </div>
                  <button
                    onClick={handleViewAllActivity}
                    className="text-xs font-bold text-primary hover:text-primary/80 transition-colors"
                  >
                    View Full Audit Log
                  </button>
                </div>
                <div className="divide-y divide-slate-100 dark:divide-slate-800">
                  {MOCK_ACTIVITIES.map((item) => (
                    <div
                      key={item.id}
                      className="p-4 flex items-center space-x-4 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-all group"
                    >
                      <div className="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center flex-shrink-0 group-hover:scale-110 transition-transform">
                        {item.type === "user" && (
                          <UserCirclePlus size={20} className="text-blue-500" />
                        )}
                        {item.type === "alert" && (
                          <Warning size={20} className="text-rose-500" />
                        )}
                        {item.type === "system" && (
                          <CloudArrowDown size={20} className="text-emerald-500" />
                        )}
                        {item.type === "marketing" && (
                          <EnvelopeSimple size={20} className="text-amber-500" />
                        )}
                      </div>
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2">
                          <p className="text-sm font-bold text-slate-900 dark:text-slate-50 truncate">
                            {item.title}
                          </p>
                          {item.badge && (
                            <span
                              className={`text-[9px] font-black px-1.5 py-0.5 rounded uppercase tracking-tighter ${getBadgeStyles(
                                item.type
                              )}`}
                            >
                              {item.badge}
                            </span>
                          )}
                        </div>
                        <p className="text-xs text-slate-500 dark:text-slate-400 truncate">
                          {item.description}
                        </p>
                      </div>
                      <div className="text-right flex-shrink-0">
                        <p className="text-[10px] font-bold text-slate-400 uppercase">
                          {item.time}
                        </p>
                        <button className="text-[10px] font-bold text-primary opacity-0 group-hover:opacity-100 transition-opacity">
                          Details →
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Weekly Growth Trends Chart */}
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <div className="flex justify-between items-center mb-8">
                  <div>
                    <h3 className="font-bold text-slate-900 dark:text-slate-50">
                      Weekly Growth Trends
                    </h3>
                    <p className="text-[10px] text-slate-400 font-bold uppercase tracking-widest">User Acquisition</p>
                  </div>
                  <div className="flex items-center gap-6">
                    <span className="flex items-center gap-2">
                      <div className="size-2 rounded-full bg-primary" />
                      <span className="text-[10px] font-bold text-slate-500 uppercase">Pro</span>
                    </span>
                    <span className="flex items-center gap-2">
                      <div className="size-2 rounded-full bg-slate-200 dark:bg-slate-700" />
                      <span className="text-[10px] font-bold text-slate-500 uppercase">Free</span>
                    </span>
                  </div>
                </div>
                <div className="flex items-end justify-between gap-2 h-48 px-2">
                  {[
                    { label: "Mon", free: 40, pro: 20 },
                    { label: "Tue", free: 45, pro: 25 },
                    { label: "Wed", free: 35, pro: 15 },
                    { label: "Thu", free: 50, pro: 35 },
                    { label: "Fri", free: 60, pro: 45 },
                    { label: "Sat", free: 55, pro: 30 },
                    { label: "Sun", free: 58, pro: 40 },
                  ].map((bar, index) => (
                    <div key={index} className="flex-1 flex flex-col items-center gap-2 h-full group">
                      <div className="w-full max-w-[32px] flex-1 bg-slate-50 dark:bg-slate-800/50 rounded-t-lg relative overflow-hidden">
                        {/* Free Bar */}
                        <div 
                          className="absolute bottom-0 w-full bg-slate-200 dark:bg-slate-700 transition-all duration-500 group-hover:bg-slate-300"
                          style={{ height: `${bar.free}%` }}
                        />
                        {/* Pro Bar */}
                        <div 
                          className="absolute bottom-0 w-full bg-primary transition-all duration-700 delay-100 group-hover:brightness-110"
                          style={{ height: `${bar.pro}%` }}
                        />
                      </div>
                      <span className="text-[10px] font-bold text-slate-400 uppercase tracking-tighter">{bar.label}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            {/* Secondary Column (Right 1/3) */}
            <div className="space-y-6">
              {/* Plan Distribution */}
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <h3 className="font-bold text-slate-900 dark:text-slate-50 mb-8">
                  Plan Distribution
                </h3>
                <div className="relative w-44 h-44 mx-auto mb-10">
                  <svg
                    className="w-full h-full transform -rotate-90 filter drop-shadow-sm"
                    viewBox="0 0 36 36"
                  >
                    <circle cx="18" cy="18" r="16" fill="transparent" stroke="currentColor" strokeWidth="3" className="text-slate-100 dark:text-slate-800" />
                    <circle cx="18" cy="18" r="16" fill="transparent" stroke="currentColor" strokeWidth="3" strokeDasharray="65, 100" strokeLinecap="round" className="text-primary" />
                    <circle cx="18" cy="18" r="16" fill="transparent" stroke="currentColor" strokeWidth="3" strokeDasharray="15, 100" strokeDashoffset="-65" strokeLinecap="round" className="text-slate-400" />
                  </svg>
                  <div className="absolute inset-0 flex flex-col items-center justify-center">
                    <span className="text-3xl font-black text-slate-900 dark:text-slate-50 tracking-tighter">
                      85%
                    </span>
                    <div className="flex items-center gap-1">
                      <div className="size-1.5 rounded-full bg-emerald-500" />
                      <span className="text-[8px] font-black text-slate-400 uppercase tracking-widest">
                        HEALTHY
                      </span>
                    </div>
                  </div>
                </div>
                <div className="space-y-4">
                  {[
                    { label: "Pro Plan", value: "65%", color: "bg-primary" },
                    { label: "Lite Plan", value: "15%", color: "bg-slate-400" },
                    { label: "Free Tier", value: "20%", color: "bg-slate-200 dark:bg-slate-700" },
                  ].map((p, i) => (
                    <div key={i} className="flex justify-between items-center group cursor-pointer">
                      <div className="flex items-center gap-3">
                        <div className={`size-2.5 rounded-sm ${p.color}`} />
                        <span className="text-xs font-bold text-slate-600 dark:text-slate-400 group-hover:text-slate-900 dark:group-hover:text-slate-200 transition-colors">
                          {p.label}
                        </span>
                      </div>
                      <span className="text-xs font-black text-slate-900 dark:text-slate-50 tabular-nums">
                        {p.value}
                      </span>
                    </div>
                  ))}
                </div>
              </div>

              {/* System Health */}
              <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 p-6">
                <div className="flex items-center justify-between mb-6">
                  <h3 className="font-bold text-slate-900 dark:text-slate-50">
                    System Health
                  </h3>
                  <span className="text-[9px] font-black bg-emerald-50 dark:bg-emerald-900/20 text-emerald-600 px-2 py-0.5 rounded-full border border-emerald-100 dark:border-emerald-800">
                    ALL SYSTEMS NOMINAL
                  </span>
                </div>
                <div className="space-y-5">
                  {[
                    { label: "Cloud Storage", value: 72, color: "bg-primary" },
                    { label: "API Uptime", value: 99.9, color: "bg-emerald-500" },
                    { label: "Worker Load", value: 45, color: "bg-amber-500" },
                  ].map((s, i) => (
                    <div key={i}>
                      <div className="flex justify-between text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase mb-2 tracking-tighter">
                        <span>{s.label}</span>
                        <span className="text-slate-900 dark:text-slate-100 font-black">{s.value}%</span>
                      </div>
                      <div className="w-full bg-slate-100 dark:bg-slate-800 h-2 rounded-full overflow-hidden">
                        <div
                          className={`${s.color} h-full rounded-full transition-all duration-1000 ease-out`}
                          style={{ width: `${s.value}%` }}
                        />
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Quick Support Card */}
              <div className="bg-slate-900 dark:bg-slate-800 rounded-xl shadow-xl p-6 text-white overflow-hidden relative border border-slate-800">
                <div className="relative z-10">
                  <div className="size-10 rounded-lg bg-white/10 flex items-center justify-center mb-4">
                    <CheckCircle size={24} weight="duotone" className="text-primary-light" />
                  </div>
                  <h4 className="font-black text-lg mb-1">Enterprise Support</h4>
                  <p className="text-slate-400 text-[11px] mb-6 leading-relaxed">
                    Direct access to security audits and platform integrations.
                  </p>
                  <button
                    onClick={handleContactSupport}
                    className="w-full bg-white text-slate-900 hover:bg-slate-100 py-2.5 rounded-lg font-black text-xs transition-all active:scale-95 shadow-lg"
                  >
                    Open Ticket
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
