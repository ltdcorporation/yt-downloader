"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError } from "@/lib/api";
import {
  TrayArrowDown,
  Layout,
  Users,
  ChartBar,
  Gear,
  Plus,
  SignOut,
  MagnifyingGlass,
  Bell,
  Question,
  User,
  SealCheck,
  DownloadSimple,
  CurrencyDollar,
  UserCirclePlus,
  Warning,
  CloudArrowDown,
  EnvelopeSimple,
  CheckCircle,
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
  const [searchQuery, setSearchQuery] = useState("");
  const [activeNavItem, setActiveNavItem] = useState("dashboard");

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
    if (!isAuthChecking && !currentUser) {
      router.push("/");
      return;
    }
    setIsPageLoading(false);
  }, [currentUser, isAuthChecking, router]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  const handleNewReport = () => {
    console.log("New report");
  };

  const handleViewAllActivity = () => {
    console.log("View all activity");
  };

  const handleContactSupport = () => {
    console.log("Contact support");
  };

  if (isAuthChecking || isPageLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-surface-container-lowest dark:bg-slate-950">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!currentUser) {
    return null;
  }

  const navItems = [
    { id: "dashboard", icon: Layout, label: "Dashboard", active: true },
    { id: "users", icon: Users, label: "Users", active: false },
    { id: "analytics", icon: ChartBar, label: "Analytics", active: false },
    { id: "settings", icon: Gear, label: "Settings", active: false },
  ];

  const getBadgeStyles = (type: ActivityItem["type"]) => {
    switch (type) {
      case "user":
        return "bg-primary/10 text-primary";
      case "alert":
        return "bg-error/10 text-error";
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
    <div className="flex h-screen overflow-hidden bg-background dark:bg-slate-950">
      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 border-r border-slate-200 dark:border-slate-800 bg-surface-container-low dark:bg-slate-950 flex flex-col p-4 space-y-2 transition-transform duration-300 ${
          isSidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
        }`}
      >
        <div className="px-2 py-4 flex items-center space-x-3 mb-6">
          <div className="w-10 h-10 rounded-lg bg-primary flex items-center justify-center shadow-lg shadow-primary/20">
            <TrayArrowDown size={24} weight="fill" className="text-white" />
          </div>
          <div>
            <h1 className="font-black text-slate-900 dark:text-slate-50 tracking-tighter">
              QuickSnap
            </h1>
            <p className="text-[10px] font-bold text-slate-500 uppercase tracking-widest">
              Admin Console
            </p>
          </div>
        </div>

        <nav className="flex-1 space-y-1">
          {navItems.map((item) => (
            <button
              key={item.id}
              onClick={() => setActiveNavItem(item.id)}
              className={`w-full flex items-center space-x-3 px-4 py-2.5 rounded-lg transition-all duration-150 ease-in-out ${
                item.active
                  ? "bg-primary/10 text-primary font-bold"
                  : "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-900"
              }`}
            >
              <item.icon size={20} weight={item.active ? "fill" : "regular"} />
              <span className="text-sm">{item.label}</span>
            </button>
          ))}
        </nav>

        <div className="pt-4 border-t border-slate-200 dark:border-slate-800">
          <button
            onClick={handleNewReport}
            className="w-full bg-primary text-white py-2 rounded-lg font-bold text-sm shadow-lg shadow-primary/20 active:scale-95 transition-all flex items-center justify-center space-x-2"
          >
            <Plus size={18} weight="bold" />
            <span>New Report</span>
          </button>
          <button
            onClick={handleLogout}
            className="mt-4 w-full flex items-center space-x-3 px-4 py-2.5 text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 transition-all duration-150"
          >
            <SignOut size={20} />
            <span className="text-sm">Logout</span>
          </button>
        </div>
      </aside>

      {/* Mobile Overlay */}
      {isSidebarOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/50 backdrop-blur-sm lg:hidden"
          onClick={() => setIsSidebarOpen(false)}
        />
      )}

      {/* Main Content */}
      <main className="flex-1 ml-0 lg:ml-64 min-h-screen">
        {/* Top App Bar */}
        <header className="fixed top-0 right-0 left-0 lg:left-64 h-16 bg-white/80 dark:bg-slate-900/80 backdrop-blur-md border-b border-slate-200 dark:border-slate-800 z-40 px-6 flex justify-between items-center shadow-sm">
          <div className="flex items-center space-x-4">
            {/* Mobile Menu Button */}
            <button
              onClick={() => setIsSidebarOpen(true)}
              className="lg:hidden p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors"
              aria-label="Open menu"
            >
              <Layout size={24} />
            </button>

            {/* Search Bar */}
            <div className="relative group">
              <span className="absolute inset-y-0 left-3 flex items-center text-slate-400">
                <MagnifyingGlass size={20} weight="bold" />
              </span>
              <input
                className="pl-10 pr-4 py-1.5 bg-slate-100 dark:bg-slate-800 border-none rounded-lg text-sm focus:ring-2 focus:ring-primary/50 w-48 sm:w-64 transition-all text-slate-900 dark:text-slate-100 placeholder-slate-400"
                placeholder="Search data points..."
                type="text"
                value={searchQuery}
                onChange={(e) => setSearchQuery(e.target.value)}
              />
            </div>
          </div>

          <div className="flex items-center space-x-4">
            {/* Notifications */}
            <button className="p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors relative">
              <Bell size={24} />
              <span className="absolute top-2 right-2 w-2 h-2 bg-error rounded-full"></span>
            </button>

            {/* Help */}
            <button className="p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors">
              <Question size={24} />
            </button>

            <div className="h-8 w-px bg-slate-200 dark:bg-slate-700 mx-2"></div>

            {/* User Profile */}
            <div className="flex items-center space-x-3">
              <div className="text-right hidden sm:block">
                <p className="text-xs font-black text-slate-900 dark:text-slate-100">
                  {currentUser.full_name}
                </p>
                <p className="text-[10px] text-slate-500 dark:text-slate-400 font-medium">
                  Super Admin
                </p>
              </div>
              <div className="w-10 h-10 rounded-full border-2 border-white dark:border-slate-700 shadow-sm bg-primary/10 flex items-center justify-center overflow-hidden">
                {currentUser.avatar_url ? (
                  // eslint-disable-next-line @next/next/no-img-element
                  <img
                    src={currentUser.avatar_url}
                    alt={currentUser.full_name}
                    className="w-full h-full object-cover"
                  />
                ) : (
                  <User size={24} className="text-primary" />
                )}
              </div>
            </div>
          </div>
        </header>

        {/* Content Canvas */}
        <div className="pt-24 px-4 sm:px-8 pb-12">
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
                className="bg-surface dark:bg-slate-900 p-6 rounded-xl shadow-sm border border-transparent hover:border-slate-200 dark:hover:border-slate-700 transition-all group"
              >
                <div className="flex justify-between items-start mb-4">
                  <div className="p-2 bg-primary/10 rounded-lg text-primary">
                    <metric.icon size={24} weight="fill" />
                  </div>
                  <span
                    className={`text-xs font-bold px-2 py-1 rounded ${getTrendBadgeStyles(
                      metric.trendDirection
                    )}`}
                  >
                    {metric.trend}
                  </span>
                </div>
                <h3 className="text-slate-500 dark:text-slate-400 text-xs font-bold uppercase tracking-wider mb-1">
                  {metric.title}
                </h3>
                <p className="text-2xl font-black text-slate-900 dark:text-slate-50">
                  {metric.value}
                </p>
              </div>
            ))}
          </div>

          {/* Main Dashboard Layout */}
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-8">
            {/* Activity Feed (Left 2/3) */}
            <div className="lg:col-span-2 space-y-6">
              {/* Recent Activity */}
              <div className="bg-surface dark:bg-slate-900 rounded-xl shadow-sm border border-slate-100 dark:border-slate-800 overflow-hidden">
                <div className="px-6 py-4 border-b border-slate-100 dark:border-slate-800 flex justify-between items-center">
                  <h3 className="font-bold text-slate-900 dark:text-slate-50">
                    Recent Activity
                  </h3>
                  <button
                    onClick={handleViewAllActivity}
                    className="text-xs font-bold text-primary hover:underline"
                  >
                    View All
                  </button>
                </div>
                <div className="divide-y divide-slate-50 dark:divide-slate-800">
                  {MOCK_ACTIVITIES.map((item) => (
                    <div
                      key={item.id}
                      className="p-4 flex items-center space-x-4 hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors"
                    >
                      <div className="w-10 h-10 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center flex-shrink-0">
                        {item.type === "user" && (
                          <UserCirclePlus size={20} className="text-slate-500" />
                        )}
                        {item.type === "alert" && (
                          <Warning size={20} className="text-slate-500" />
                        )}
                        {item.type === "system" && (
                          <CloudArrowDown size={20} className="text-slate-500" />
                        )}
                        {item.type === "marketing" && (
                          <EnvelopeSimple size={20} className="text-slate-500" />
                        )}
                      </div>
                      <div className="flex-1">
                        <p className="text-sm font-medium text-slate-900 dark:text-slate-50">
                          {item.title}
                        </p>
                        <p className="text-xs text-slate-500 dark:text-slate-400">
                          {item.description}
                        </p>
                      </div>
                      <div className="text-right">
                        <p className="text-[10px] font-bold text-slate-400">
                          {item.time}
                        </p>
                        {item.badge && (
                          <span
                            className={`text-[10px] font-black px-2 py-0.5 rounded uppercase ${getBadgeStyles(
                              item.type
                            )}`}
                          >
                            {item.badge}
                          </span>
                        )}
                      </div>
                    </div>
                  ))}
                </div>
              </div>

              {/* Weekly Growth Trends Chart */}
              <div className="bg-surface dark:bg-slate-900 rounded-xl shadow-sm border border-slate-100 dark:border-slate-800 p-6">
                <div className="flex justify-between items-center mb-6">
                  <h3 className="font-bold text-slate-900 dark:text-slate-50">
                    Weekly Growth Trends
                  </h3>
                  <div className="flex space-x-4">
                    <span className="flex items-center space-x-1">
                      <span className="w-3 h-3 rounded-full bg-primary"></span>
                      <span className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">
                        Pro Users
                      </span>
                    </span>
                    <span className="flex items-center space-x-1">
                      <span className="w-3 h-3 rounded-full bg-slate-200 dark:bg-slate-700"></span>
                      <span className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase">
                        Free Tier
                      </span>
                    </span>
                  </div>
                </div>
                <div className="flex-1 flex items-end justify-between space-x-2 h-48">
                  {[
                    { free: 24, pro: 12 },
                    { free: 28, pro: 16 },
                    { free: 20, pro: 8 },
                    { free: 24, pro: 20 },
                    { free: 32, pro: 24 },
                    { free: 28, pro: 18 },
                    { free: 30, pro: 22 },
                  ].map((bar, index) => (
                    <div
                      key={index}
                      className="w-full bg-slate-50 dark:bg-slate-800 rounded-t-sm relative group h-32"
                    >
                      <div
                        className="absolute bottom-0 w-full bg-slate-200 dark:bg-slate-700 rounded-t-sm group-hover:bg-slate-300 dark:group-hover:bg-slate-600 transition-colors"
                        style={{ height: `${bar.free}%` }}
                      ></div>
                      <div
                        className="absolute bottom-0 w-full bg-primary rounded-t-sm transition-all"
                        style={{ height: `${bar.pro}%` }}
                      ></div>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            {/* Secondary Column (Right 1/3) */}
            <div className="space-y-6">
              {/* Plan Distribution */}
              <div className="bg-surface dark:bg-slate-900 rounded-xl shadow-sm border border-slate-100 dark:border-slate-800 p-6">
                <h3 className="font-bold text-slate-900 dark:text-slate-50 mb-6">
                  Plan Distribution
                </h3>
                <div className="relative w-40 h-40 mx-auto mb-8">
                  <svg
                    className="w-full h-full transform -rotate-90"
                    viewBox="0 0 36 36"
                  >
                    <path
                      className="text-slate-100 dark:text-slate-800"
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                      fill="none"
                      stroke="currentColor"
                      strokeWidth="4"
                    />
                    <path
                      className="text-primary"
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                      fill="none"
                      stroke="currentColor"
                      strokeDasharray="65, 100"
                      strokeLinecap="round"
                      strokeWidth="4"
                    />
                    <path
                      className="text-slate-300 dark:text-slate-700"
                      d="M18 2.0845 a 15.9155 15.9155 0 0 1 0 31.831 a 15.9155 15.9155 0 0 1 0 -31.831"
                      fill="none"
                      stroke="currentColor"
                      strokeDasharray="15, 100"
                      strokeDashoffset="-65"
                      strokeLinecap="round"
                      strokeWidth="4"
                    />
                  </svg>
                  <div className="absolute inset-0 flex flex-col items-center justify-center">
                    <span className="text-xl font-black text-slate-900 dark:text-slate-50">
                      85%
                    </span>
                    <span className="text-[8px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-widest">
                      Growth
                    </span>
                  </div>
                </div>
                <div className="space-y-3">
                  <div className="flex justify-between items-center">
                    <div className="flex items-center space-x-2">
                      <span className="w-2 h-2 rounded-full bg-primary"></span>
                      <span className="text-xs font-medium text-slate-700 dark:text-slate-300">
                        Pro Plan
                      </span>
                    </div>
                    <span className="text-xs font-bold text-slate-900 dark:text-slate-50">
                      65%
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <div className="flex items-center space-x-2">
                      <span className="w-2 h-2 rounded-full bg-slate-300 dark:bg-slate-700"></span>
                      <span className="text-xs font-medium text-slate-700 dark:text-slate-300">
                        Lite Plan
                      </span>
                    </div>
                    <span className="text-xs font-bold text-slate-900 dark:text-slate-50">
                      15%
                    </span>
                  </div>
                  <div className="flex justify-between items-center">
                    <div className="flex items-center space-x-2">
                      <span className="w-2 h-2 rounded-full bg-slate-100 dark:bg-slate-800"></span>
                      <span className="text-xs font-medium text-slate-700 dark:text-slate-300">
                        Free Tier
                      </span>
                    </div>
                    <span className="text-xs font-bold text-slate-900 dark:text-slate-50">
                      20%
                    </span>
                  </div>
                </div>
              </div>

              {/* Quick Support Card */}
              <div className="bg-primary rounded-xl shadow-lg shadow-primary/20 p-6 text-white overflow-hidden relative">
                <div className="relative z-10">
                  <h4 className="font-black text-lg mb-2">Need Assistance?</h4>
                  <p className="text-white/80 text-xs mb-4 leading-relaxed">
                    Contact our enterprise support team for direct platform
                    integrations or security audits.
                  </p>
                  <button
                    onClick={handleContactSupport}
                    className="bg-white/20 hover:bg-white/30 backdrop-blur-sm text-white px-4 py-2 rounded-lg font-bold text-xs transition-colors"
                  >
                    Contact Support
                  </button>
                </div>
                <CheckCircle
                  size={128}
                  weight="fill"
                  className="absolute -bottom-4 -right-4 text-white/10"
                />
              </div>

              {/* System Health */}
              <div className="bg-surface dark:bg-slate-900 rounded-xl shadow-sm border border-slate-100 dark:border-slate-800 p-6">
                <h3 className="font-bold text-slate-900 dark:text-slate-50 mb-4">
                  System Health
                </h3>
                <div className="space-y-4">
                  <div>
                    <div className="flex justify-between text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase mb-1">
                      <span>Cloud Storage</span>
                      <span>72%</span>
                    </div>
                    <div className="w-full bg-slate-100 dark:bg-slate-800 h-1.5 rounded-full">
                      <div
                        className="bg-primary h-full rounded-full"
                        style={{ width: "72%" }}
                      ></div>
                    </div>
                  </div>
                  <div>
                    <div className="flex justify-between text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase mb-1">
                      <span>API Uptime</span>
                      <span>99.9%</span>
                    </div>
                    <div className="w-full bg-slate-100 dark:bg-slate-800 h-1.5 rounded-full">
                      <div
                        className="bg-emerald-400 h-full rounded-full"
                        style={{ width: "99%" }}
                      ></div>
                    </div>
                  </div>
                </div>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
