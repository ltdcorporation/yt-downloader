"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { usePathname, useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type MaintenanceServiceItem,
  type MaintenanceServiceKey,
  type MaintenanceServiceStatus,
  type MaintenanceSnapshotResponse,
} from "@/lib/api";
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
  Cpu,
  WarningCircle,
  Pulse,
} from "@phosphor-icons/react";

interface SystemHealthCard {
  key: MaintenanceServiceKey;
  name: string;
  status: MaintenanceServiceStatus;
  enabled: boolean;
  icon: React.ElementType;
}

interface MaintenanceUIBaseline {
  version: number;
  updatedAt: string;
  updatedByUserID: string;
  enabled: boolean;
  estimatedDowntime: string;
  publicMessage: string;
  services: SystemHealthCard[];
}

const SERVICE_ICON_MAP: Record<MaintenanceServiceKey, React.ElementType> = {
  api_gateway: Cpu,
  primary_database: Database,
  worker_nodes: Pulse,
};

const SERVICE_ORDER: MaintenanceServiceKey[] = [
  "api_gateway",
  "primary_database",
  "worker_nodes",
];

function normalizeServices(services: MaintenanceServiceItem[]): SystemHealthCard[] {
  const byKey = new Map<MaintenanceServiceKey, MaintenanceServiceItem>();
  services.forEach((service) => {
    byKey.set(service.key, service);
  });

  return SERVICE_ORDER.map((key) => {
    const service = byKey.get(key);
    return {
      key,
      name:
        service?.name ||
        (key === "api_gateway"
          ? "API Gateway"
          : key === "primary_database"
            ? "Primary Database"
            : "Worker Nodes"),
      status: service?.status || "active",
      enabled: service?.enabled ?? true,
      icon: SERVICE_ICON_MAP[key],
    };
  });
}

function getStatusBadgeStyles(status: MaintenanceServiceStatus) {
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
}

function formatRelativeTime(timestamp: string): string {
  const parsed = new Date(timestamp);
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

export default function AdminMaintenancePage() {
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
  const [isSaving, setIsSaving] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");

  const [snapshotVersion, setSnapshotVersion] = useState<number>(1);
  const [lastUpdatedAt, setLastUpdatedAt] = useState("");
  const [lastUpdatedBy, setLastUpdatedBy] = useState("");

  const [globalMaintenanceEnabled, setGlobalMaintenanceEnabled] = useState(false);
  const [systemHealth, setSystemHealth] = useState<SystemHealthCard[]>(
    normalizeServices([]),
  );
  const [estimatedDowntime, setEstimatedDowntime] = useState("45 minutes");
  const [publicMessage, setPublicMessage] = useState("");

  const [baseline, setBaseline] = useState<MaintenanceUIBaseline | null>(null);

  const applySnapshotToState = useCallback((snapshot: MaintenanceSnapshotResponse) => {
    const normalizedServices = normalizeServices(snapshot.maintenance.services);

    setSnapshotVersion(snapshot.meta.version);
    setLastUpdatedAt(snapshot.meta.updated_at);
    setLastUpdatedBy(snapshot.meta.updated_by_user_id || "");

    setGlobalMaintenanceEnabled(snapshot.maintenance.enabled);
    setEstimatedDowntime(snapshot.maintenance.estimated_downtime);
    setPublicMessage(snapshot.maintenance.public_message);
    setSystemHealth(normalizedServices);

    setBaseline({
      version: snapshot.meta.version,
      updatedAt: snapshot.meta.updated_at,
      updatedByUserID: snapshot.meta.updated_by_user_id || "",
      enabled: snapshot.maintenance.enabled,
      estimatedDowntime: snapshot.maintenance.estimated_downtime,
      publicMessage: snapshot.maintenance.public_message,
      services: normalizedServices,
    });
  }, []);

  const loadMaintenance = useCallback(async () => {
    setLoadError("");
    try {
      const snapshot = await api.getAdminMaintenance();
      applySnapshotToState(snapshot);
    } catch (error) {
      const message =
        error instanceof APIError
          ? error.message
          : "Failed to load maintenance state";
      setLoadError(message);
    } finally {
      setIsPageLoading(false);
    }
  }, [applySnapshotToState]);

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
    if (isAuthChecking) {
      return;
    }

    if (!currentUser || currentUser.role !== "admin") {
      router.push("/");
      return;
    }

    void loadMaintenance();
  }, [currentUser, isAuthChecking, loadMaintenance, router]);

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
    if (!baseline) {
      return false;
    }

    if (baseline.enabled !== globalMaintenanceEnabled) {
      return true;
    }
    if (baseline.estimatedDowntime !== estimatedDowntime.trim()) {
      return true;
    }
    if (baseline.publicMessage !== publicMessage.trim()) {
      return true;
    }

    if (baseline.services.length !== systemHealth.length) {
      return true;
    }

    for (let index = 0; index < systemHealth.length; index += 1) {
      const current = systemHealth[index];
      const previous = baseline.services[index];
      if (
        !previous ||
        previous.key !== current.key ||
        previous.status !== current.status ||
        previous.enabled !== current.enabled
      ) {
        return true;
      }
    }

    return false;
  }, [
    baseline,
    estimatedDowntime,
    globalMaintenanceEnabled,
    publicMessage,
    systemHealth,
  ]);

  const handleSaveChanges = async () => {
    setIsSaving(true);
    setLoadError("");
    setActionMessage("");

    try {
      const snapshot = await api.updateAdminMaintenance({
        maintenance: {
          enabled: globalMaintenanceEnabled,
          estimated_downtime: estimatedDowntime.trim(),
          public_message: publicMessage.trim(),
          services: systemHealth.map((service) => ({
            key: service.key,
            status: service.status,
            enabled: service.enabled,
          })),
        },
        meta: {
          version: snapshotVersion,
        },
      });

      applySnapshotToState(snapshot);
      setActionMessage("Maintenance configuration updated.");
    } catch (error) {
      if (
        error instanceof APIError &&
        error.code === "maintenance_version_conflict"
      ) {
        setLoadError("Settings changed elsewhere. Reloaded latest version.");
        await loadMaintenance();
      } else {
        const message =
          error instanceof APIError
            ? error.message
            : "Failed to save maintenance changes";
        setLoadError(message);
      }
    } finally {
      setIsSaving(false);
    }
  };

  const handleDiscard = () => {
    if (!baseline) {
      return;
    }

    setSnapshotVersion(baseline.version);
    setLastUpdatedAt(baseline.updatedAt);
    setLastUpdatedBy(baseline.updatedByUserID);
    setGlobalMaintenanceEnabled(baseline.enabled);
    setEstimatedDowntime(baseline.estimatedDowntime);
    setPublicMessage(baseline.publicMessage);
    setSystemHealth(baseline.services);
    setActionMessage("Changes discarded.");
    setLoadError("");
  };

  const toggleSystemHealth = (key: MaintenanceServiceKey) => {
    setSystemHealth((prev) =>
      prev.map((card) =>
        card.key === key ? { ...card, enabled: !card.enabled } : card,
      ),
    );
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

        <div className="max-w-6xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-6">
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
                <WarningCircle
                  size={20}
                  className="text-amber-600 dark:text-amber-400 animate-pulse"
                />
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
                      Activating this will return 503 maintenance responses on
                      user routes.
                    </p>
                  </div>
                  <label className="relative inline-flex items-center cursor-pointer">
                    <input
                      checked={globalMaintenanceEnabled}
                      onChange={(e) =>
                        setGlobalMaintenanceEnabled(e.target.checked)
                      }
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
                      key={card.key}
                      className={`bg-white dark:bg-slate-900 border border-slate-200 dark:border-slate-800 p-6 rounded-xl transition-all hover:shadow-md ${
                        !card.enabled ? "opacity-60 grayscale" : ""
                      }`}
                    >
                      <div className="flex items-start justify-between mb-6">
                        <div
                          className={`p-3 rounded-xl ${
                            card.enabled
                              ? "bg-primary/10 text-primary"
                              : "bg-slate-100 dark:bg-slate-800 text-slate-400"
                          }`}
                        >
                          <card.icon
                            size={24}
                            weight={card.enabled ? "fill" : "regular"}
                          />
                        </div>
                        <label className="relative inline-flex items-center cursor-pointer scale-90">
                          <input
                            checked={card.enabled}
                            onChange={() => toggleSystemHealth(card.key)}
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
                          <div
                            className={`size-1.5 rounded-full ${
                              card.enabled
                                ? "bg-emerald-500 animate-pulse shadow-[0_0_8px_rgba(16,185,129,0.6)]"
                                : "bg-slate-400"
                            }`}
                          />
                          <span
                            className={`text-[10px] font-black uppercase tracking-wider px-2 py-0.5 rounded border ${getStatusBadgeStyles(
                              card.status,
                            )}`}
                          >
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
                    maxLength={1000}
                  />
                  <div className="mt-3 flex justify-between text-[10px] font-bold text-slate-400 uppercase tracking-widest px-1">
                    <span>Plain text message</span>
                    <span>{publicMessage.length}/1000</span>
                  </div>
                </div>
              </div>

              {/* Tips Card */}
              <div className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-xl border border-slate-800 relative overflow-hidden group">
                <Pulse
                  size={80}
                  weight="thin"
                  className="absolute -bottom-4 -right-4 text-white/5 group-hover:scale-110 transition-transform duration-700"
                />
                <h4 className="font-bold text-sm mb-3 flex items-center gap-2 text-primary-light">
                  <Info size={18} />
                  Maintenance Tip
                </h4>
                <p className="text-slate-400 text-xs leading-relaxed mb-4">
                  Keep the public message short and clear. Mention estimated
                  downtime to reduce support tickets.
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
                Last updated by{" "}
                <span className="text-slate-900 dark:text-white">
                  {lastUpdatedBy || "system"}
                </span>{" "}
                {lastUpdatedAt ? formatRelativeTime(lastUpdatedAt) : "just now"}
              </span>
            </div>
            <div className="flex items-center gap-3 w-full sm:w-auto">
              <button
                onClick={handleDiscard}
                disabled={!isDirty || isSaving}
                className="flex-1 sm:flex-none px-6 py-2.5 text-xs font-black text-slate-500 hover:text-slate-900 dark:hover:text-white transition-colors disabled:opacity-40 disabled:cursor-not-allowed"
              >
                DISCARD
              </button>
              <button
                onClick={handleSaveChanges}
                disabled={isSaving || !isDirty}
                className="flex-1 sm:flex-none px-8 py-2.5 bg-primary text-white text-xs font-black rounded-lg shadow-lg shadow-primary/20 hover:brightness-110 transition-all active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed flex items-center justify-center gap-2 uppercase tracking-widest"
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
