"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError, type AuthUser, type UserPlan } from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import {
  Layout,
  Users,
  Gear,
  CheckCircle,
  PencilSimple,
  Prohibit,
  FileCsv,
  UserPlus,
  CaretLeft,
  CaretRight,
  Wrench,
} from "@phosphor-icons/react";

interface UserData {
  id: string;
  name: string;
  email: string;
  avatar?: string;
  joinDate: string;
  plan: UserPlan | "suspended";
  role: string;
  downloads: number;
}

const USERS_PER_PAGE = 20;

export default function AdminUsersPage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [currentPage, setCurrentPage] = useState(1);
  const [users, setUsers] = useState<UserData[]>([]);
  const [totalUsers, setTotalUsers] = useState(0);
  const [loadError, setLoadError] = useState("");

  const fetchUsers = useCallback(async (page: number) => {
    setIsPageLoading(true);
    setLoadError("");
    try {
      const offset = (page - 1) * USERS_PER_PAGE;
      const response = await api.listAdminUsers(USERS_PER_PAGE, offset);
      
      const mappedUsers: UserData[] = response.items.map((u: AuthUser) => ({
        id: u.id,
        name: u.full_name,
        email: u.email,
        avatar: u.avatar_url,
        joinDate: new Date(u.created_at).toLocaleDateString("en-US", {
          month: "short",
          day: "2-digit",
          year: "numeric",
        }),
        plan: u.plan,
        role: u.role,
        downloads: 0, // Backend doesn't have download count yet per user in auth
      }));

      setUsers(mappedUsers);
      setTotalUsers(response.page.total);
    } catch (error) {
      setLoadError(error instanceof Error ? error.message : "Failed to load users");
    } finally {
      setIsPageLoading(false);
    }
  }, []);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
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
    
    if (currentUser && currentUser.role !== "admin") {
      router.push("/");
      return;
    }

    if (currentUser) {
      void fetchUsers(currentPage);
    }
  }, [currentUser, isAuthChecking, router, currentPage, fetchUsers]);

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
      case "monthly":
      case "weekly":
      case "daily":
        return "bg-primary/10 text-primary";
      case "free":
        return "bg-slate-100 text-slate-500 dark:bg-slate-800 dark:text-slate-400";
      case "suspended":
        return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400";
      default:
        return "bg-slate-100 text-slate-500";
    }
  };

  const getPlanLabel = (plan: UserData["plan"]) => {
    switch (plan) {
      case "monthly":
        return "Monthly Pro";
      case "weekly":
        return "Weekly Pro";
      case "daily":
        return "Daily Pro";
      case "free":
        return "Free Tier";
      case "suspended":
        return "Suspended";
      default:
        return plan;
    }
  };

  if (isAuthChecking || (isPageLoading && users.length === 0)) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
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

  const totalPages = Math.ceil(totalUsers / USERS_PER_PAGE);
  const startIndex = (currentPage - 1) * USERS_PER_PAGE + 1;
  const endIndex = Math.min(currentPage * USERS_PER_PAGE, totalUsers);

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
          title="User Management"
          showText={false}
        />

        <div className="max-w-7xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-8">
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
                className="px-4 py-2 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 font-bold text-sm rounded-lg border border-slate-200 dark:border-slate-700 transition-all active:scale-95 hover:bg-slate-50 dark:hover:bg-slate-700 flex items-center gap-2 shadow-sm"
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

          {loadError && (
            <div className="p-4 bg-rose-50 border border-rose-100 rounded-lg text-rose-600 text-sm font-medium">
              {loadError}
            </div>
          )}

          <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden relative">
            {isPageLoading && (
              <div className="absolute inset-0 bg-white/50 dark:bg-slate-900/50 flex items-center justify-center z-10">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
              </div>
            )}
            
            <div className="overflow-x-auto">
              <table className="w-full text-left border-collapse">
                <thead>
                  <tr className="bg-slate-50/50 dark:bg-slate-800/30">
                    <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                      User
                    </th>
                    <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                      Join Date
                    </th>
                    <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                      Status
                    </th>
                    <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800">
                      Role
                    </th>
                    <th className="px-6 py-4 text-[10px] font-black text-slate-500 dark:text-slate-400 uppercase tracking-widest border-b border-slate-100 dark:border-slate-800 text-right">
                      Actions
                    </th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                  {users.length === 0 && !isPageLoading ? (
                    <tr>
                      <td colSpan={5} className="px-6 py-12 text-center text-slate-500 dark:text-slate-400 text-sm italic">
                        No users found.
                      </td>
                    </tr>
                  ) : (
                    users.map((user) => (
                      <tr
                        key={user.id}
                        className="hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors group cursor-pointer"
                        onClick={() => router.push(`/admin/users/${user.id}`)}
                      >
                        <td className="px-6 py-4">
                          <div className="flex items-center gap-3">
                            <div className="w-10 h-10 rounded-full bg-slate-200 dark:bg-slate-700 flex items-center justify-center overflow-hidden border border-slate-200 dark:border-slate-700">
                              {user.avatar ? (
                                // eslint-disable-next-line @next/next/no-img-element
                                <img
                                  src={user.avatar}
                                  alt={user.name}
                                  className="w-full h-full object-cover"
                                />
                              ) : (
                                <span className="text-slate-500 dark:text-slate-400 font-bold text-xs uppercase">
                                  {user.name.substring(0, 2)}
                                </span>
                              )}
                            </div>
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
                        <td className="px-6 py-4">
                          <span className="text-xs font-bold text-slate-600 dark:text-slate-400 uppercase tracking-tighter">
                            {user.role}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-right">
                          <div className="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity" onClick={(e) => e.stopPropagation()}>
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
                                className="p-2 text-slate-400 hover:text-error hover:bg-rose-50 dark:hover:bg-rose-950/30 rounded-md transition-all"
                                aria-label="Block user"
                              >
                                <Prohibit size={20} />
                              </button>
                            )}
                          </div>
                        </td>
                      </tr>
                    )
                  ))}
                </tbody>
              </table>
            </div>

            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/30 border-t border-slate-100 dark:border-slate-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <p className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-widest">
                Showing {totalUsers === 0 ? 0 : startIndex}-{endIndex} of {totalUsers} users
              </p>
              <div className="flex items-center gap-1">
                <button
                  onClick={() => setCurrentPage((p) => Math.max(1, p - 1))}
                  disabled={currentPage === 1}
                  className="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-700 hover:shadow-sm transition-all text-slate-400 disabled:opacity-30 disabled:cursor-not-allowed"
                  aria-label="Previous page"
                >
                  <CaretLeft size={20} weight="bold" />
                </button>
                <div className="flex items-center gap-1">
                   {[...Array(totalPages)].map((_, i) => (
                     <button
                       key={i + 1}
                       onClick={() => setCurrentPage(i + 1)}
                       className={`w-9 h-9 rounded-lg font-bold text-xs transition-all ${
                         currentPage === i + 1
                           ? "bg-primary text-white shadow-sm"
                           : "border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
                       }`}
                     >
                       {i + 1}
                     </button>
                   ))}
                </div>
                <button
                  onClick={() => setCurrentPage((p) => Math.min(totalPages, p + 1))}
                  disabled={currentPage === totalPages || totalPages === 0}
                  className="p-1.5 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 hover:bg-slate-50 dark:hover:bg-slate-700 hover:shadow-sm transition-all text-slate-400 disabled:opacity-30 disabled:cursor-not-allowed"
                  aria-label="Next page"
                >
                  <CaretRight size={20} weight="bold" />
                </button>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
