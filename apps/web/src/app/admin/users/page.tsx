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
  PencilSimple,
  Prohibit,
  FileCsv,
  UserPlus,
  Lightbulb,
  ShieldCheck,
  CaretLeft,
  CaretRight,
} from "@phosphor-icons/react";

interface UserData {
  id: string;
  name: string;
  email: string;
  avatar?: string;
  initials?: string;
  joinDate: string;
  plan: "pro" | "free" | "suspended";
  downloads: number;
}

interface TipCard {
  id: string;
  type: "tip" | "security";
  title: string;
  description: string;
}

const MOCK_USERS: UserData[] = [
  {
    id: "1",
    name: "Sarah Jenkins",
    email: "sarah.j@example.com",
    avatar: "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=100&h=100&fit=crop",
    joinDate: "Oct 24, 2023",
    plan: "pro",
    downloads: 1240,
  },
  {
    id: "2",
    name: "Marcus Thorne",
    email: "m.thorne@quickmail.net",
    avatar: "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=100&h=100&fit=crop",
    joinDate: "Nov 12, 2023",
    plan: "free",
    downloads: 85,
  },
  {
    id: "3",
    name: "Elena Rodriguez",
    email: "elena.rod@webflow.io",
    avatar: "https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=100&h=100&fit=crop",
    joinDate: "Dec 05, 2023",
    plan: "pro",
    downloads: 4521,
  },
  {
    id: "4",
    name: "James Harrison",
    email: "jharrison@corporate.com",
    initials: "JH",
    joinDate: "Jan 18, 2024",
    plan: "suspended",
    downloads: 0,
  },
];

const MOCK_TIPS: TipCard[] = [
  {
    id: "1",
    type: "tip",
    title: "Pro Tip: Bulk Actions",
    description:
      "Select multiple users using the checkboxes (visible on hover) to perform bulk subscription upgrades or suspension actions at once.",
  },
  {
    id: "2",
    type: "security",
    title: "Security Audit",
    description:
      "Your last security review was 12 days ago. It is recommended to run a permission audit for high-volume download accounts monthly.",
  },
];

export default function AdminUsersPage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [searchQuery, setSearchQuery] = useState("");
  const [activeNavItem, setActiveNavItem] = useState("users");
  const [currentPage, setCurrentPage] = useState(1);
  const [users, setUsers] = useState<UserData[]>(MOCK_USERS);

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

  const handleExportCSV = () => {
    console.log("Export CSV");
  };

  const handleAddUser = () => {
    console.log("Add new user");
  };

  const handleEditUser = (userId: string) => {
    console.log("Edit user:", userId);
  };

  const handleBlockUser = (userId: string) => {
    console.log("Block user:", userId);
  };

  const handleUnblockUser = (userId: string) => {
    console.log("Unblock user:", userId);
  };

  const getPlanBadgeStyles = (plan: UserData["plan"]) => {
    switch (plan) {
      case "pro":
        return "bg-primary/10 text-primary";
      case "free":
        return "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400";
      case "suspended":
        return "bg-error-container text-error dark:bg-error/20 dark:text-error";
    }
  };

  const getPlanLabel = (plan: UserData["plan"]) => {
    switch (plan) {
      case "pro":
        return "Pro Plan";
      case "free":
        return "Free Tier";
      case "suspended":
        return "Suspended";
    }
  };

  const navItems = [
    { id: "dashboard", icon: Layout, label: "Dashboard", active: false },
    { id: "users", icon: Users, label: "Users", active: true },
    { id: "analytics", icon: ChartBar, label: "Analytics", active: false },
    { id: "settings", icon: Gear, label: "Settings", active: false },
  ];

  const totalPages = 1248;
  const startIndex = (currentPage - 1) * 10 + 1;
  const endIndex = Math.min(currentPage * 10, 12482);

  return (
    <div className="flex h-screen overflow-hidden bg-background dark:bg-slate-950">
      {/* Sidebar */}
      <aside
        className={`fixed inset-y-0 left-0 z-50 w-64 border-r border-slate-200 dark:border-slate-800 bg-surface-container-low dark:bg-slate-950 flex flex-col p-4 space-y-2 transition-transform duration-300 ${
          isSidebarOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"
        }`}
      >
        <div className="px-2 py-4 flex items-center space-x-3 mb-6">
          <div className="w-8 h-8 rounded-lg bg-primary flex items-center justify-center shadow-lg shadow-primary/20">
            <TrayArrowDown size={20} weight="fill" className="text-white" />
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
              className={`w-full flex items-center gap-3 px-3 py-2 rounded-lg transition-all duration-150 text-sm font-medium ${
                item.active
                  ? "bg-primary/10 text-primary font-bold"
                  : "text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-200 hover:bg-slate-100 dark:hover:bg-slate-900"
              }`}
            >
              <item.icon size={20} weight={item.active ? "fill" : "regular"} />
              <span>{item.label}</span>
            </button>
          ))}
        </nav>

        <div className="pt-4 border-t border-slate-200 dark:border-slate-800">
          <button
            onClick={handleAddUser}
            className="w-full flex items-center justify-center gap-2 bg-primary text-white py-2 rounded-lg shadow-lg shadow-primary/20 font-bold text-sm active:scale-95 transition-all"
          >
            <Plus size={18} weight="bold" />
            <span>New Report</span>
          </button>
          <button
            onClick={handleLogout}
            className="flex items-center gap-3 px-3 py-2 mt-4 text-slate-600 dark:text-slate-400 hover:text-error transition-all duration-150 text-sm font-medium w-full"
          >
            <SignOut size={20} />
            <span>Logout</span>
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
        <header className="fixed top-0 right-0 left-0 lg:left-64 h-16 bg-white/80 dark:bg-slate-900/80 backdrop-blur-md border-b border-slate-200 dark:border-slate-800 z-40">
          <div className="flex justify-between items-center px-6 lg:px-8 h-full">
            <div className="flex items-center gap-4">
              {/* Mobile Menu Button */}
              <button
                onClick={() => setIsSidebarOpen(true)}
                className="lg:hidden p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors"
                aria-label="Open menu"
              >
                <Layout size={24} />
              </button>

              {/* Search Bar */}
              <div className="relative">
                <MagnifyingGlass
                  size={20}
                  weight="bold"
                  className="absolute left-3 top-1/2 -translate-y-1/2 text-slate-400"
                />
                <input
                  className="pl-10 pr-4 py-1.5 bg-slate-100 dark:bg-slate-800 border-none rounded-md text-sm w-48 sm:w-64 focus:ring-2 focus:ring-primary/50 transition-all text-slate-900 dark:text-slate-100 placeholder-slate-400"
                  placeholder="Global search..."
                  type="text"
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                />
              </div>
            </div>

            <div className="flex items-center gap-4">
              {/* Notifications */}
              <button className="p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors relative">
                <Bell size={24} />
                <span className="absolute top-2 right-2 w-2 h-2 bg-error rounded-full border-2 border-white dark:border-slate-700"></span>
              </button>

              {/* Help */}
              <button className="p-2 text-slate-500 hover:bg-slate-50 dark:hover:bg-slate-800 rounded-full transition-colors">
                <Question size={24} />
              </button>

              <div className="h-8 w-px bg-slate-200 dark:bg-slate-700 mx-2"></div>

              {/* User Profile */}
              <div className="flex items-center gap-3">
                <div className="text-right hidden sm:block">
                  <p className="text-xs font-bold text-slate-900 dark:text-slate-100">
                    {currentUser.full_name}
                  </p>
                  <p className="text-[10px] text-slate-500 dark:text-slate-400">
                    System Admin
                  </p>
                </div>
                <div className="w-8 h-8 rounded-full border border-slate-200 dark:border-slate-700 shadow-sm bg-primary/10 flex items-center justify-center overflow-hidden">
                  {currentUser.avatar_url ? (
                    // eslint-disable-next-line @next/next/no-img-element
                    <img
                      src={currentUser.avatar_url}
                      alt={currentUser.full_name}
                      className="w-full h-full object-cover"
                    />
                  ) : (
                    <User size={20} className="text-primary" />
                  )}
                </div>
              </div>
            </div>
          </div>
        </header>

        {/* Main Content Canvas */}
        <main className="ml-0 lg:ml-64 pt-16 min-h-screen p-4 sm:p-8">
          <div className="max-w-7xl mx-auto space-y-8">
            {/* Page Header */}
            <div className="flex flex-col sm:flex-row justify-between items-start sm:items-end gap-4">
              <div>
                <h2 className="text-2xl font-black text-slate-900 dark:text-slate-50 tracking-tight">
                  User Management
                </h2>
                <p className="text-slate-500 dark:text-slate-400 text-sm mt-1 font-medium">
                  Manage permissions, subscription tiers, and account health for
                  all registered users.
                </p>
              </div>
              <div className="flex gap-3">
                <button
                  onClick={handleExportCSV}
                  className="px-4 py-2 bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-slate-100 font-bold text-sm rounded-lg transition-all active:scale-95 hover:bg-slate-200 dark:hover:bg-slate-700 flex items-center gap-2"
                >
                  <FileCsv size={20} weight="bold" />
                  <span className="hidden sm:inline">Export CSV</span>
                </button>
                <button
                  onClick={handleAddUser}
                  className="px-4 py-2 bg-primary text-white font-bold text-sm rounded-lg shadow-lg shadow-primary/20 transition-all active:scale-95 flex items-center gap-2"
                >
                  <UserPlus size={20} weight="bold" />
                  <span className="hidden sm:inline">Add New User</span>
                </button>
              </div>
            </div>

            {/* Main Table Section */}
            <div className="bg-surface dark:bg-slate-900 rounded-lg shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden">
              {/* Table Content */}
              <div className="overflow-x-auto">
                <table className="w-full text-left border-collapse">
                  <thead>
                    <tr className="bg-surface-container-low dark:bg-slate-800/50">
                      <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                        User
                      </th>
                      <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                        Join Date
                      </th>
                      <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                        Status
                      </th>
                      <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800 text-center">
                        Downloads
                      </th>
                      <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800 text-right">
                        Actions
                      </th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                    {users.map((user) => (
                      <tr
                        key={user.id}
                        className="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors group"
                      >
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-3">
                            {user.avatar ? (
                              // eslint-disable-next-line @next/next/no-img-element
                              <img
                                src={user.avatar}
                                alt={user.name}
                                className="w-10 h-10 rounded-full border border-slate-200 dark:border-slate-700 object-cover"
                              />
                            ) : (
                              <div className="w-10 h-10 rounded-full bg-slate-200 dark:bg-slate-700 flex items-center justify-center text-slate-500 dark:text-slate-400 font-bold text-xs">
                                {user.initials}
                              </div>
                            )}
                            <div>
                              <p className="text-sm font-bold text-slate-900 dark:text-slate-50">
                                {user.name}
                              </p>
                              <p className="text-xs text-slate-500 dark:text-slate-400">
                                {user.email}
                              </p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-sm text-slate-600 dark:text-slate-300 font-medium">
                          {user.joinDate}
                        </td>
                        <td className="px-6 py-4">
                          <span
                            className={`inline-flex items-center px-2.5 py-0.5 rounded text-[10px] font-black uppercase tracking-tight ${getPlanBadgeStyles(
                              user.plan
                            )}`}
                          >
                            {getPlanLabel(user.plan)}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-sm text-slate-900 dark:text-slate-50 font-bold text-center">
                          {user.downloads.toLocaleString()}
                        </td>
                        <td className="px-6 py-4 text-right">
                          <div className="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity">
                            <button
                              onClick={() => handleEditUser(user.id)}
                              className="p-2 text-slate-400 hover:text-primary hover:bg-primary/5 dark:hover:bg-primary/10 rounded-md transition-all"
                              aria-label="Edit user"
                            >
                              <PencilSimple size={20} />
                            </button>
                            {user.plan === "suspended" ? (
                              <button
                                onClick={() => handleUnblockUser(user.id)}
                                className="p-2 text-slate-400 hover:text-emerald-600 hover:bg-emerald-50 dark:hover:bg-emerald-950/30 rounded-md transition-all"
                                aria-label="Unblock user"
                              >
                                <CheckCircle size={20} weight="fill" />
                              </button>
                            ) : (
                              <button
                                onClick={() => handleBlockUser(user.id)}
                                className="p-2 text-slate-400 hover:text-error hover:bg-error-container/5 dark:hover:bg-error/20 rounded-md transition-all"
                                aria-label="Block user"
                              >
                                <Prohibit size={20} />
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>

              {/* Pagination */}
              <div className="px-6 py-4 bg-surface-container-low dark:bg-slate-800/50 border-t border-slate-100 dark:border-slate-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
                <p className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-widest">
                  Showing {startIndex}-{endIndex} of 12,482 users
                </p>
                <div className="flex items-center gap-1">
                  <button
                    onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                    disabled={currentPage === 1}
                    className="p-1.5 rounded hover:bg-white dark:hover:bg-slate-700 hover:shadow-sm transition-all text-slate-400 disabled:opacity-30 disabled:cursor-not-allowed"
                    aria-label="Previous page"
                  >
                    <CaretLeft size={20} weight="bold" />
                  </button>
                  <button className="w-8 h-8 rounded bg-primary text-white font-bold text-xs shadow-sm">
                    1
                  </button>
                  <button className="w-8 h-8 rounded hover:bg-white dark:hover:bg-slate-700 hover:shadow-sm text-slate-600 dark:text-slate-300 font-bold text-xs transition-all">
                    2
                  </button>
                  <button className="w-8 h-8 rounded hover:bg-white dark:hover:bg-slate-700 hover:shadow-sm text-slate-600 dark:text-slate-300 font-bold text-xs transition-all">
                    3
                  </button>
                  <span className="px-2 text-slate-400">...</span>
                  <button className="w-8 h-8 rounded hover:bg-white dark:hover:bg-slate-700 hover:shadow-sm text-slate-600 dark:text-slate-300 font-bold text-xs transition-all">
                    {totalPages}
                  </button>
                  <button
                    onClick={() =>
                      setCurrentPage((p) => Math.min(totalPages, p + 1))
                    }
                    className="p-1.5 rounded hover:bg-white dark:hover:bg-slate-700 hover:shadow-sm transition-all text-slate-400"
                    aria-label="Next page"
                  >
                    <CaretRight size={20} weight="bold" />
                  </button>
                </div>
              </div>
            </div>

            {/* Contextual Help / Tips */}
            <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
              {MOCK_TIPS.map((tip) => (
                <div
                  key={tip.id}
                  className={`p-6 rounded-lg border flex gap-4 items-start ${
                    tip.type === "tip"
                      ? "bg-primary/5 border-primary/10"
                      : "bg-surface-container-low dark:bg-slate-900 border-slate-200 dark:border-slate-800"
                  }`}
                >
                  <div
                    className={`w-10 h-10 rounded-full flex items-center justify-center shrink-0 ${
                      tip.type === "tip"
                        ? "bg-primary/10"
                        : "bg-slate-200 dark:bg-slate-800"
                    }`}
                  >
                    {tip.type === "tip" ? (
                      <Lightbulb
                        size={20}
                        className="text-primary"
                        weight="fill"
                      />
                    ) : (
                      <ShieldCheck
                        size={20}
                        className="text-slate-500 dark:text-slate-400"
                        weight="fill"
                      />
                    )}
                  </div>
                  <div>
                    <h4
                      className={`text-sm font-bold tracking-tight ${
                        tip.type === "tip"
                          ? "text-primary"
                          : "text-slate-900 dark:text-slate-50"
                      }`}
                    >
                      {tip.title}
                    </h4>
                    <p className="text-xs text-slate-600 dark:text-slate-400 mt-1 leading-relaxed">
                      {tip.description}
                    </p>
                  </div>
                </div>
              ))}
            </div>
          </div>
        </main>
      </main>
    </div>
  );
}
