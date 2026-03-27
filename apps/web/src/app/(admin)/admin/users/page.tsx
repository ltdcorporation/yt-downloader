"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
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
  Crown,
  User as UserIcon,
  FileCsv,
  CaretLeft,
  CaretRight,
  PencilSimple,
  ArrowClockwise,
  Wrench,
} from "@phosphor-icons/react";

interface UserData {
  id: string;
  fullName: string;
  email: string;
  avatarURL?: string;
  createdAt: string;
  plan: UserPlan;
  role: UserRole;
}

const USERS_PER_PAGE = 10;

function formatDate(dateString: string): string {
  const parsed = new Date(dateString);
  if (Number.isNaN(parsed.getTime())) {
    return "-";
  }
  return parsed.toLocaleDateString("en-US", {
    month: "short",
    day: "2-digit",
    year: "numeric",
  });
}

function getPlanBadgeStyles(plan: UserPlan): string {
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

function getPlanLabel(plan: UserPlan): string {
  switch (plan) {
    case "monthly":
      return "Monthly";
    case "weekly":
      return "Weekly";
    case "daily":
      return "Daily";
    default:
      return "Free";
  }
}

function roleBadgeStyles(role: UserRole): string {
  return role === "admin"
    ? "bg-primary/10 text-primary"
    : "bg-slate-100 text-slate-600 dark:bg-slate-800 dark:text-slate-300";
}

function mapUsers(items: AuthUser[]): UserData[] {
  return items.map((user) => ({
    id: user.id,
    fullName: user.full_name,
    email: user.email,
    avatarURL: user.avatar_url,
    createdAt: user.created_at,
    plan: user.plan,
    role: user.role,
  }));
}

export default function AdminUsersPage() {
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
  const [currentPage, setCurrentPage] = useState(1);
  const [users, setUsers] = useState<UserData[]>([]);
  const [totalUsers, setTotalUsers] = useState(0);
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [updatingRoleUserID, setUpdatingRoleUserID] = useState<string | null>(
    null,
  );

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

  const fetchUsers = useCallback(async (page: number) => {
    setIsPageLoading(true);
    setLoadError("");

    try {
      const offset = (page - 1) * USERS_PER_PAGE;
      const response = await api.listAdminUsers(USERS_PER_PAGE, offset);
      setUsers(mapUsers(response.items));
      setTotalUsers(response.page.total);
    } catch (error) {
      const message =
        error instanceof APIError
          ? error.message
          : "Failed to load users";
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
      router.push("/");
      return;
    }

    void fetchUsers(currentPage);
  }, [currentUser, fetchUsers, currentPage, isAuthChecking, router]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  const handleRefresh = async () => {
    setActionMessage("");
    await fetchUsers(currentPage);
  };

  const handleExportCSV = () => {
    if (users.length === 0) {
      return;
    }

    const header = ["id", "full_name", "email", "role", "plan", "created_at"];
    const rows = users.map((user) => [
      user.id,
      user.fullName,
      user.email,
      user.role,
      user.plan,
      user.createdAt,
    ]);

    const toCell = (value: string) => `"${value.replace(/"/g, '""')}"`;
    const csv = [header, ...rows]
      .map((row) => row.map((value) => toCell(String(value))).join(","))
      .join("\n");

    const blob = new Blob([csv], { type: "text/csv;charset=utf-8" });
    const url = URL.createObjectURL(blob);
    const link = document.createElement("a");
    link.href = url;
    link.download = `admin-users-page-${currentPage}.csv`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  };

  const handleToggleRole = async (user: UserData) => {
    if (updatingRoleUserID) {
      return;
    }

    const nextRole: UserRole = user.role === "admin" ? "user" : "admin";

    setUpdatingRoleUserID(user.id);
    setActionMessage("");
    setLoadError("");

    try {
      const updated = await api.updateAdminUser(user.id, { role: nextRole });
      setUsers((prev) =>
        prev.map((item) =>
          item.id === user.id
            ? {
                ...item,
                role: updated.role,
                plan: updated.plan,
              }
            : item,
        ),
      );
      setActionMessage(
        `${updated.full_name} is now ${updated.role === "admin" ? "an admin" : "a user"}.`,
      );

      if (currentUser?.id === updated.id) {
        setCurrentUser({
          ...currentUser,
          role: updated.role,
          plan: updated.plan,
          plan_expires_at: updated.plan_expires_at,
        });
      }
    } catch (error) {
      const message =
        error instanceof APIError
          ? error.message
          : "Failed to update user role";
      setLoadError(message);
    } finally {
      setUpdatingRoleUserID(null);
    }
  };

  const totalPages = Math.max(1, Math.ceil(totalUsers / USERS_PER_PAGE));
  const startIndex = totalUsers === 0 ? 0 : (currentPage - 1) * USERS_PER_PAGE + 1;
  const endIndex = Math.min(currentPage * USERS_PER_PAGE, totalUsers);

  const userProfile = useMemo(() => {
    if (!currentUser) {
      return {
        name: "",
        email: "",
        plan: "",
        avatar: DEFAULT_AVATAR_URL,
      };
    }

    return {
      name: currentUser.full_name,
      email: currentUser.email,
      plan: "Super Admin",
      avatar: currentUser.avatar_url || DEFAULT_AVATAR_URL,
    };
  }, [currentUser]);

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
                Manage roles and plans for registered accounts.
              </p>
            </div>
            <div className="flex gap-3">
              <button
                onClick={handleRefresh}
                className="px-4 py-2 bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100 font-bold text-sm rounded-lg border border-slate-200 dark:border-slate-700 transition-all active:scale-95 hover:bg-slate-50 dark:hover:bg-slate-700 flex items-center gap-2 shadow-sm"
              >
                <ArrowClockwise size={18} weight="bold" />
                <span className="hidden sm:inline">Refresh</span>
              </button>
              <button
                onClick={handleExportCSV}
                disabled={users.length === 0}
                className="px-4 py-2 bg-primary text-white font-bold text-sm rounded-lg shadow-lg shadow-primary/20 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed flex items-center gap-2"
              >
                <FileCsv size={20} weight="bold" />
                <span className="hidden sm:inline">Export CSV</span>
              </button>
            </div>
          </div>

          {loadError ? (
            <div className="p-4 bg-rose-50 border border-rose-100 rounded-lg text-rose-600 text-sm font-medium">
              {loadError}
            </div>
          ) : null}

          {actionMessage ? (
            <div className="p-4 bg-emerald-50 border border-emerald-100 rounded-lg text-emerald-700 text-sm font-medium">
              {actionMessage}
            </div>
          ) : null}

          <div className="bg-white dark:bg-slate-900 rounded-xl shadow-sm border border-slate-200 dark:border-slate-800 overflow-hidden relative">
            {isPageLoading ? (
              <div className="absolute inset-0 bg-white/50 dark:bg-slate-900/50 flex items-center justify-center z-10">
                <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
              </div>
            ) : null}

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
                      Plan
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
                      <td
                        colSpan={5}
                        className="px-6 py-12 text-center text-slate-500 dark:text-slate-400 text-sm italic"
                      >
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
                              {user.avatarURL ? (
                                // eslint-disable-next-line @next/next/no-img-element
                                <img
                                  src={user.avatarURL}
                                  alt={user.fullName}
                                  className="w-full h-full object-cover"
                                />
                              ) : (
                                <span className="text-slate-500 dark:text-slate-400 font-bold text-xs uppercase">
                                  {user.fullName.substring(0, 2)}
                                </span>
                              )}
                            </div>
                            <div>
                              <p className="text-sm font-bold text-slate-900 dark:text-slate-50">
                                {user.fullName}
                              </p>
                              <p className="text-xs text-slate-500 dark:text-slate-400">
                                {user.email}
                              </p>
                            </div>
                          </div>
                        </td>
                        <td className="px-6 py-4 text-sm text-slate-600 dark:text-slate-300 font-medium">
                          {formatDate(user.createdAt)}
                        </td>
                        <td className="px-6 py-4">
                          <span
                            className={`inline-flex items-center px-2.5 py-0.5 rounded text-[10px] font-black uppercase tracking-tight ${getPlanBadgeStyles(
                              user.plan,
                            )}`}
                          >
                            {getPlanLabel(user.plan)}
                          </span>
                        </td>
                        <td className="px-6 py-4">
                          <span
                            className={`inline-flex items-center gap-1 text-xs font-bold uppercase px-2 py-0.5 rounded ${roleBadgeStyles(
                              user.role,
                            )}`}
                          >
                            {user.role === "admin" ? (
                              <Crown size={12} weight="fill" />
                            ) : (
                              <UserIcon size={12} weight="fill" />
                            )}
                            {user.role}
                          </span>
                        </td>
                        <td className="px-6 py-4 text-right">
                          <div
                            className="flex justify-end gap-2 opacity-0 group-hover:opacity-100 transition-opacity"
                            onClick={(event) => event.stopPropagation()}
                          >
                            <button
                              onClick={() =>
                                router.push(`/admin/users/${encodeURIComponent(user.id)}`)
                              }
                              className="p-2 text-slate-400 hover:text-primary hover:bg-primary/5 dark:hover:bg-primary/10 rounded-md transition-all"
                              aria-label="Edit user"
                            >
                              <PencilSimple size={18} />
                            </button>
                            <button
                              onClick={() => void handleToggleRole(user)}
                              disabled={updatingRoleUserID === user.id}
                              className="px-2.5 py-1.5 text-[10px] font-black uppercase rounded border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-300 hover:bg-slate-100 dark:hover:bg-slate-800 disabled:opacity-50 disabled:cursor-not-allowed"
                            >
                              {updatingRoleUserID === user.id
                                ? "Updating"
                                : user.role === "admin"
                                  ? "Make User"
                                  : "Make Admin"}
                            </button>
                          </div>
                        </td>
                      </tr>
                    ))
                  )}
                </tbody>
              </table>
            </div>

            <div className="px-6 py-4 bg-slate-50/50 dark:bg-slate-800/30 border-t border-slate-100 dark:border-slate-800 flex flex-col sm:flex-row items-start sm:items-center justify-between gap-4">
              <p className="text-[10px] font-bold text-slate-500 dark:text-slate-400 uppercase tracking-widest">
                Showing {startIndex}-{endIndex} of {totalUsers} users
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
                  {Array.from({ length: totalPages }).map((_, index) => {
                    const page = index + 1;
                    return (
                      <button
                        key={page}
                        onClick={() => setCurrentPage(page)}
                        className={`w-9 h-9 rounded-lg font-bold text-xs transition-all ${
                          currentPage === page
                            ? "bg-primary text-white shadow-sm"
                            : "border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-800 text-slate-600 dark:text-slate-300 hover:bg-slate-50 dark:hover:bg-slate-700"
                        }`}
                      >
                        {page}
                      </button>
                    );
                  })}
                </div>
                <button
                  onClick={() =>
                    setCurrentPage((p) => Math.min(totalPages, p + 1))
                  }
                  disabled={currentPage === totalPages}
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
