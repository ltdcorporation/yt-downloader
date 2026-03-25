"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, usePathname } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError } from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import {
  Layout,
  Users,
  Gear,
  Wrench,
  Database,
  Monitor,
  Clock,
  Info,
  FloppyDisk,
  ArrowCounterClockwise,
  Cpu,
  WarningCircle,
  CheckCircle,
  Pulse,
} from "@phosphor-icons/react";

interface SystemHealthCard {
  id: string;
  name: string;
  status: "active" | "maintenance" | "scaling" | "refreshing";
  enabled: boolean;
  icon: React.ElementType;
}

const INITIAL_SYSTEM_HEALTH: SystemHealthCard[] = [
  {
    id: "1",
    name: "API Gateway",
    status: "active",
    enabled: true,
    icon: Cpu,
  },
  {
    id: "2",
    name: "Primary Database",
    status: "active",
    enabled: true,
    icon: Database,
  },
  {
    id: "3",
    name: "Worker Nodes",
    status: "active",
    enabled: true,
    icon: Pulse,
  },
];

export default function AdminMaintenancePage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [globalMaintenanceEnabled, setGlobalMaintenanceEnabled] = useState(false);
  const [systemHealth, setSystemHealth] = useState<SystemHealthCard[]>(INITIAL_SYSTEM_HEALTH);
  const [estimatedDowntime, setEstimatedDowntime] = useState("45 minutes");
  const [publicMessage, setPublicMessage] = useState(
    "Our systems are currently undergoing a scheduled core infrastructure update. We expect to be back online shortly. Thank you for your patience."
  );
  const [isSaving, setIsSaving] = useState(false);

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

  const handleSaveChanges = async () => {
    setIsSaving(true);
    await new Promise((resolve) => setTimeout(resolve, 1000));
    setIsSaving(false);
  };

  const handleDiscard = () => {
    setGlobalMaintenanceEnabled(false);
    setSystemHealth(INITIAL_SYSTEM_HEALTH);
    setEstimatedDowntime("45 minutes");
    setPublicMessage(
      "Our systems are currently undergoing a scheduled core infrastructure update. We expect to be back online shortly. Thank you for your patience."
    );
  };

  const toggleSystemHealth = (id: string) => {
    setSystemHealth((prev) =>
      prev.map((card) =>
        card.id === id ? { ...card, enabled: !card.enabled } : card
      )
    );
  };

  const getStatusBadgeStyles = (status: SystemHealthCard["status"]) => {
    switch (status) {
      case "maintenance":
        return "text-amber-600 bg-amber-50 dark:bg-amber-900/20 dark:text-amber-400 border-amber-100 dark:border-amber-800";
      case "scaling":
        return "text-blue-600 bg-blue-50 dark:bg-blue-900/20 dark:text-blue-400 border-blue-100 dark:border-blue-800";
      case "active":
        return "text-emerald-600 bg-emerald-50 dark:bg-emerald-900/20 dark:text-emerald-400 border-emerald-100 dark:border-emerald-800";
      default:
        return "text-slate-600 bg-slate-50 dark:bg-slate-900/20 dark:text-slate-400 border-slate-100 dark:border-slate-800";
    }
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
          title="Maintenance"
          showText={false}
        />

        <div className="max-w-6xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-8">
          {/* Page Header */}
          <div className="flex flex-col sm:flex-row justify-between items-start sm:items-end gap-4">
            <div>
              <h2 className="text-2xl font-black text-slate-900 dark:text-slate-50 tracking-tight">
                Maintenance Console
              </h2>
              <p className="text-slate-500 dark:text-slate-400 text-sm mt-1 font-medium">
                Control service availability and system-wide maintenance states.
              </p>
            </div>
            {globalMaintenanceEnabled && (
              <div className="flex items-center gap-2 px-4 py-2 bg-amber-50 dark:bg-amber-900/20 border border-amber-200 dark:border-amber-800 rounded-xl">
                <WarningCircle size={20} className="text-amber-600 dark:text-amber-400 animate-pulse" />
                <span className="text-xs font-black text-amber-700 dark:text-amber-400 uppercase tracking-tight">
                  System Restricted
                </span>
              </div>
            )}
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
            <div className="col-span-12 lg:col-span-8 space-y-8">
              {/* Global Toggle Card */}
              <section className="bg-white dark:bg-slate-900 rounded-xl p-8 border border-slate-200 dark:border-slate-800 shadow-sm relative overflow-hidden group">
                <div className="absolute top-0 left-0 w-1 h-full bg-primary group-hover:w-2 transition-all" />
                <div className="flex justify-between items-center relative z-10">
                  <div className="max-w-md">
                    <h3 className="text-xl font-bold text-slate-900 dark:text-white">
                      Global Maintenance Mode
                    </h3>
                    <p className="text-sm text-slate-500 dark:text-slate-400 mt-2 leading-relaxed">
                      Activating this will immediately redirect all users to a maintenance page. APIs will return a 503 Service Unavailable status.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      checked={globalMaintenanceEnabled}
                      onChange={(e) => setGlobalMaintenanceEnabled(e.target.checked)}
                      className="sr-only peer"
                      type="checkbox"
                    />
                    <div className="w-14 h-7 bg-slate-200 dark:bg-slate-800 peer-focus:ring-4 peer-focus:ring-primary/20 rounded-full peer peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[4px] after:left-[4px] after:bg-white after:rounded-full after:h-[21px] after:w-[21px] after:transition-all peer-checked:bg-primary shadow-inner transition-colors"></div>
                  </label>
                </div>
              </section>

              {/* Service Health Grid */}
              <div className="space-y-4">
                <h4 className="text-xs font-black text-slate-400 dark:text-slate-500 uppercase tracking-[0.2em]">
                  Service-Level Overrides
                </h4>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
                  {systemHealth.map((card) => (
                    <div
                      key={card.id}
                      className={`bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 p-6 rounded-xl transition-all hover:shadow-md ${!card.enabled ? 'opacity-60 grayscale' : ''}`}
                    >
                      <div className="flex items-start justify-between mb-6">
                        <div className={`p-3 rounded-xl ${card.enabled ? 'bg-primary/10 text-primary' : 'bg-slate-100 dark:bg-slate-800 text-slate-400'}`}>
                          <card.icon size={24} weight={card.enabled ? "fill" : "regular"} />
                        </div>
                        <label className="relative inline-flex items-center cursor-pointer scale-90">
                          <input
                            checked={card.enabled}
                            onChange={() => toggleSystemHealth(card.id)}
                            className="sr-only peer"
                            type="checkbox"
                          />
                          <div className="w-11 h-6 bg-slate-200 dark:bg-slate-800 peer-checked:bg-emerald-500 rounded-full peer peer-checked:after:translate-x-full after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:rounded-full after:h-5 after:w-5 after:transition-all shadow-inner transition-colors"></div>
                        </label>
                      </div>
                      <div>
                        <p className="text-sm font-black text-slate-900 dark:text-slate-50">
                          {card.name}
                        </p>
                        <div className="flex items-center gap-2 mt-2">
                          <div className={`size-1.5 rounded-full ${card.enabled ? 'bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.6)]' : 'bg-slate-400'}`} />
                          <span className={`text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded border ${getStatusBadgeStyles(card.status)}`}>
                            {card.status}
                          </span>
                        </div>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            </div>

            <div className="col-span-12 lg:col-span-4 space-y-6">
              {/* Settings Card */}
              <div className="bg-white dark:bg-slate-900 rounded-xl p-6 border border-slate-200 dark:border-slate-800 shadow-sm space-y-6">
                <div>
                  <h4 className="text-sm font-bold text-slate-900 dark:text-slate-50 mb-4 flex items-center gap-2">
                    <Clock size={18} className="text-primary" />
                    Estimated Downtime
                  </h4>
                  <input
                    className="w-full px-4 py-2.5 bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-lg text-sm focus:ring-2 focus:ring-primary/50 outline-none transition-all font-bold text-slate-900 dark:text-white"
                    type="text"
                    value={estimatedDowntime}
                    onChange={(e) => setEstimatedDowntime(e.target.value)}
                  />
                </div>

                <div>
                  <h4 className="text-sm font-bold text-slate-900 dark:text-slate-50 mb-4 flex items-center gap-2">
                    <Monitor size={18} className="text-primary" />
                    Public Message
                  </h4>
                  <textarea
                    className="w-full p-4 bg-slate-50 dark:bg-slate-800/50 border border-slate-200 dark:border-slate-700 rounded-lg text-xs focus:ring-2 focus:ring-primary/50 outline-none transition-all resize-none text-slate-600 dark:text-slate-300 leading-relaxed font-medium"
                    rows={8}
                    value={publicMessage}
                    onChange={(e) => setPublicMessage(e.target.value)}
                    maxLength={500}
                  />
                  <div className="mt-3 flex justify-between text-[10px] font-bold text-slate-400 uppercase tracking-widest px-1">
                    <span>Markdown Supported</span>
                    <span>{publicMessage.length}/500</span>
                  </div>
                </div>
              </div>

              {/* Tips Card */}
              <div className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-xl border border-slate-800 relative overflow-hidden group">
                <Pulse size={80} weight="thin" className="absolute -bottom-4 -right-4 text-white/5 group-hover:scale-110 transition-transform duration-700" />
                <h4 className="font-bold text-sm mb-3 flex items-center gap-2 text-primary-light">
                  <Info size={18} />
                  Maintenance Tip
                </h4>
                <p className="text-slate-400 text-xs leading-relaxed mb-4">
                  Consider running full system diagnostics before re-enabling services to ensure data integrity.
                </p>
                <button className="text-[10px] font-black text-white bg-white/10 hover:bg-white/20 px-3 py-1.5 rounded uppercase tracking-widest transition-colors">
                  Run Pre-flight Check
                </button>
              </div>
            </div>
          </div>

          {/* Action Bar */}
          <div className="bg-white dark:bg-slate-900 p-4 border border-slate-200 dark:border-slate-800 rounded-xl flex flex-col sm:flex-row items-center justify-between gap-4 shadow-sm">
            <div className="flex items-center gap-3 text-slate-500">
              <div className="size-8 rounded-full bg-slate-100 dark:bg-slate-800 flex items-center justify-center">
                <Pulse size={16} />
              </div>
              <span className="text-xs font-bold">
                Last updated by <span className="text-slate-900 dark:text-white">admin_prime</span> 12m ago
              </span>
            </div>
            <div className="flex items-center gap-3 w-full sm:w-auto">
              <button
                onClick={handleDiscard}
                className="flex-1 sm:flex-none px-6 py-2.5 text-xs font-black text-slate-500 hover:text-slate-900 dark:hover:text-white transition-colors"
              >
                DISCARD
              </button>
              <button
                onClick={handleSaveChanges}
                disabled={isSaving}
                className="flex-1 sm:flex-none px-8 py-2.5 bg-primary text-white text-xs font-black rounded-lg shadow-lg shadow-primary/20 hover:brightness-110 transition-all active:scale-95 disabled:opacity-50 flex items-center justify-center gap-2 uppercase tracking-widest"
              >
                {isSaving ? (
                  <div className="size-3 border-2 border-white/30 border-t-white rounded-full animate-spin" />
                ) : (
                  <FloppyDisk size={16} weight="bold" />
                )}
                {isSaving ? "Saving..." : "Apply Changes"}
              </button>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
