"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import {
  api,
  APIError,
  type BillingInvoice,
  type SubscriptionDashboardResponse,
  type SubscriptionStatus,
  type UserPlan,
} from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { CreditCard, Download, CheckCircle, XCircle } from "@phosphor-icons/react";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";

const PLAN_OPTIONS: Array<{ value: UserPlan; label: string }> = [
  { value: "free", label: "Free" },
  { value: "daily", label: "Daily" },
  { value: "weekly", label: "Weekly" },
  { value: "monthly", label: "Monthly" },
];

const STATUS_LABELS: Record<SubscriptionStatus, string> = {
  active: "ACTIVE",
  inactive: "INACTIVE",
  expired: "EXPIRED",
  cancel_scheduled: "CANCEL SCHEDULED",
};

function formatPlanLabel(plan: UserPlan): string {
  switch (plan) {
    case "free":
      return "Free Plan";
    case "daily":
      return "Daily Plan";
    case "weekly":
      return "Weekly Plan";
    case "monthly":
      return "Monthly Plan";
    default:
      return plan;
  }
}

function formatMoney(amountCents: number, currency: string): string {
  try {
    return new Intl.NumberFormat("en-US", {
      style: "currency",
      currency: currency || "USD",
      minimumFractionDigits: 2,
    }).format(amountCents / 100);
  } catch {
    return `${currency || "USD"} ${(amountCents / 100).toFixed(2)}`;
  }
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
    day: "numeric",
    year: "numeric",
  });
}

function subscriptionBadgeStyles(status: SubscriptionStatus): string {
  switch (status) {
    case "active":
      return "bg-emerald-50 border-emerald-100 text-emerald-700 dark:bg-emerald-900/20 dark:border-emerald-800/60 dark:text-emerald-400";
    case "cancel_scheduled":
      return "bg-amber-50 border-amber-100 text-amber-700 dark:bg-amber-900/20 dark:border-amber-800/60 dark:text-amber-400";
    case "expired":
      return "bg-rose-50 border-rose-100 text-rose-700 dark:bg-rose-900/20 dark:border-rose-800/60 dark:text-rose-400";
    default:
      return "bg-slate-50 border-slate-200 text-slate-700 dark:bg-slate-800/60 dark:border-slate-700 dark:text-slate-300";
  }
}

function billingStatusStyles(status: BillingInvoice["status"]): string {
  switch (status) {
    case "paid":
      return "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400";
    case "pending":
      return "bg-amber-100 text-amber-700 dark:bg-amber-900/30 dark:text-amber-400";
    case "failed":
      return "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400";
    default:
      return "bg-slate-100 text-slate-700 dark:bg-slate-800 dark:text-slate-300";
  }
}

export default function SubscriptionPage() {
  const {
    currentUser,
    logout,
    isAuthChecking,
    setCurrentUser,
    setIsAuthChecking,
  } = useAuthStore();
  const router = useRouter();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isDataLoading, setIsDataLoading] = useState(false);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [actionMessage, setActionMessage] = useState("");

  const [dashboard, setDashboard] = useState<SubscriptionDashboardResponse | null>(null);
  const [billingHistory, setBillingHistory] = useState<BillingInvoice[]>([]);
  const [selectedPlan, setSelectedPlan] = useState<UserPlan>("free");
  const [isSavingPlan, setIsSavingPlan] = useState(false);
  const [isCancelling, setIsCancelling] = useState(false);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        // Session snapshot cleanup happens in navbar flow; keep store state aligned.
      }
      setCurrentUser(null);
    } finally {
      setIsAuthChecking(false);
    }
  }, [setCurrentUser, setIsAuthChecking]);

  const loadSubscriptionData = useCallback(async () => {
    setIsDataLoading(true);
    setLoadError("");

    try {
      const [subscription, history] = await Promise.all([
        api.getSubscription(),
        api.listBillingHistory({ limit: 20, offset: 0 }),
      ]);

      setDashboard(subscription);
      setBillingHistory(history.items);
      setSelectedPlan(subscription.subscription.plan);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        setCurrentUser(null);
        router.push("/");
        return;
      }

      const message =
        error instanceof APIError
          ? error.message
          : "Failed to load subscription data";
      setLoadError(message);
    } finally {
      setIsDataLoading(false);
      setIsPageLoading(false);
    }
  }, [router, setCurrentUser]);

  useEffect(() => {
    if (isAuthChecking) {
      void refreshAuthState();
    }
  }, [isAuthChecking, refreshAuthState]);

  useEffect(() => {
    if (isAuthChecking) {
      return;
    }
    if (!currentUser) {
      router.push("/");
      return;
    }

    void loadSubscriptionData();
  }, [currentUser, isAuthChecking, loadSubscriptionData, router]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    router.push("/");
  };

  const handleDownloadReceipt = (id: string) => {
    const receiptURL = api.getBillingReceiptUrl(id);
    if (typeof window !== "undefined") {
      const opened = window.open(receiptURL, "_blank", "noopener,noreferrer");
      if (!opened) {
        window.location.href = receiptURL;
      }
    }
  };

  const refreshBillingHistory = useCallback(async () => {
    const history = await api.listBillingHistory({ limit: 20, offset: 0 });
    setBillingHistory(history.items);
  }, []);

  const handleManagePlan = useCallback(async () => {
    if (!dashboard || !currentUser) {
      return;
    }

    setIsSavingPlan(true);
    setLoadError("");
    setActionMessage("");

    try {
      const updated = await api.updateSubscription({ plan: selectedPlan });
      setDashboard(updated);
      setSelectedPlan(updated.subscription.plan);
      setCurrentUser({
        ...currentUser,
        plan: updated.subscription.plan,
        plan_expires_at: updated.subscription.plan_expires_at,
      });

      await refreshBillingHistory();
      setActionMessage("Subscription updated successfully.");
    } catch (error) {
      const message =
        error instanceof APIError
          ? error.message
          : "Failed to update subscription";
      setLoadError(message);
    } finally {
      setIsSavingPlan(false);
    }
  }, [currentUser, dashboard, refreshBillingHistory, selectedPlan, setCurrentUser]);

  const handleCancelSubscription = useCallback(async () => {
    if (!dashboard || !currentUser) {
      return;
    }

    if (dashboard.subscription.plan === "free") {
      setLoadError("No active paid subscription to cancel.");
      return;
    }

    const shouldReactivate =
      dashboard.subscription.cancel_at_period_end ||
      dashboard.subscription.status === "cancel_scheduled";

    setIsCancelling(true);
    setLoadError("");
    setActionMessage("");

    try {
      const updated = shouldReactivate
        ? await api.reactivateSubscription()
        : await api.cancelSubscription({ immediate: false });

      setDashboard(updated);
      setCurrentUser({
        ...currentUser,
        plan: updated.subscription.plan,
        plan_expires_at: updated.subscription.plan_expires_at,
      });
      setActionMessage(
        shouldReactivate
          ? "Subscription reactivated."
          : "Cancellation scheduled at period end.",
      );
    } catch (error) {
      const message =
        error instanceof APIError
          ? error.message
          : "Failed to update cancellation status";
      setLoadError(message);
    } finally {
      setIsCancelling(false);
    }
  }, [currentUser, dashboard, setCurrentUser]);

  const isAdmin = currentUser?.role === "admin";

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
      plan: isAdmin ? "Super Admin" : formatPlanLabel(currentUser.plan),
      avatar: currentUser.avatar_url || DEFAULT_AVATAR_URL,
    };
  }, [currentUser, isAdmin]);

  if (isAuthChecking || isPageLoading) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  if (!currentUser || !dashboard) {
    return null;
  }

  const subscription = dashboard.subscription;
  const paymentMethod = dashboard.payment_method;
  const isPaidPlan = subscription.plan !== "free";
  const statusLabel = STATUS_LABELS[subscription.status] || subscription.status;
  const canChangePlan = selectedPlan !== subscription.plan;
  const isCancelScheduled =
    subscription.cancel_at_period_end || subscription.status === "cancel_scheduled";

  return (
    <div className="flex h-screen overflow-hidden">
      <SettingsSidebar
        user={userProfile}
        onLogout={handleLogout}
        isOpen={isSidebarOpen}
        onClose={() => setIsSidebarOpen(false)}
      />
      <main className="flex-1 overflow-y-auto bg-background-light dark:bg-background-dark">
        <SettingsHeader
          onMenuClick={() => setIsSidebarOpen(true)}
          showText={false}
        />
        <div className="max-w-4xl mx-auto px-8 pb-12 pt-2 space-y-6">
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
          <div className="flex flex-col md:flex-row md:items-end justify-between gap-4 mb-2">
            <div>
              <h1 className="text-2xl font-black tracking-tight text-slate-900 dark:text-white">
                Subscription
              </h1>
              <p className="text-slate-500 dark:text-slate-400 text-sm mt-1">
                Manage your plan, billing, and payment methods.
              </p>
            </div>
            <div
              className={`flex items-center gap-2 px-3 py-1.5 border rounded-full ${subscriptionBadgeStyles(
                subscription.status,
              )}`}
            >
              <div className="size-2 rounded-full bg-current animate-pulse" />
              <span className="text-[10px] font-bold tracking-wider">
                {statusLabel}
              </span>
            </div>
          </div>

          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6 items-start">
            {/* Left Column: Plan Details & Benefits */}
            <div className="lg:col-span-2 space-y-6">
              {/* Current Plan Card */}
              <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden shadow-sm">
                <div className="p-6 border-b border-slate-200 dark:border-slate-800">
                  <h3 className="text-lg font-bold">Current Plan</h3>
                  <p className="text-sm text-slate-500">
                    Your active subscription details and benefits.
                  </p>
                </div>
                <div className="p-6 space-y-6">
                  <div className="flex justify-between items-start gap-3">
                    <div className="flex items-center gap-4">
                      <div className="size-12 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
                        <CreditCard size={24} weight="fill" />
                      </div>
                      <div>
                        <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                          {`QuickSnap ${formatPlanLabel(subscription.plan).replace(" Plan", "")}`}
                        </h2>
                        <p className="text-sm font-medium text-slate-500 dark:text-slate-400">
                          Next billing: {formatDate(subscription.next_billing_at)}
                        </p>
                        {subscription.plan_expires_at ? (
                          <p className="text-xs text-slate-500 dark:text-slate-400 mt-1">
                            Expires: {formatDate(subscription.plan_expires_at)}
                          </p>
                        ) : null}
                      </div>
                    </div>
                    <div className="text-right">
                      <p className="text-xl font-black text-slate-900 dark:text-white">
                        {isPaidPlan
                          ? formatMoney(
                              subscription.price_cents,
                              subscription.currency,
                            )
                          : "Free"}
                      </p>
                      <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">
                        {subscription.interval === "none"
                          ? "no recurring charge"
                          : `per ${subscription.interval}`}
                      </p>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <label
                      htmlFor="plan-select"
                      className="text-xs font-bold text-slate-500 uppercase tracking-widest"
                    >
                      Change Plan
                    </label>
                    <select
                      id="plan-select"
                      value={selectedPlan}
                      onChange={(event) =>
                        setSelectedPlan(event.target.value as UserPlan)
                      }
                      className="h-9 rounded-lg border border-slate-200 dark:border-slate-700 bg-white dark:bg-slate-900 px-3 text-xs font-semibold text-slate-700 dark:text-slate-200"
                    >
                      {PLAN_OPTIONS.map((plan) => (
                        <option key={plan.value} value={plan.value}>
                          {plan.label}
                        </option>
                      ))}
                    </select>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    {subscription.benefits.map((benefit) => (
                      <div
                        key={benefit.id}
                        className="flex items-center gap-3 p-3 rounded-lg bg-slate-50 dark:bg-slate-800/50 border border-slate-100 dark:border-slate-800"
                      >
                        <CheckCircle
                          size={18}
                          weight="fill"
                          className="text-emerald-500 shrink-0"
                        />
                        <span className="text-xs font-bold text-slate-700 dark:text-slate-300">
                          {benefit.label}
                        </span>
                      </div>
                    ))}
                  </div>
                </div>
                <div className="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-800 flex justify-between items-center gap-3">
                  <p className="text-xs text-slate-500 font-medium italic">
                    {isPaidPlan
                      ? "Plan changes are applied immediately in this environment."
                      : "Upgrade anytime to unlock premium features."}
                  </p>
                  <button
                    onClick={handleManagePlan}
                    disabled={isSavingPlan || isDataLoading || !canChangePlan}
                    className="px-4 py-2 bg-primary text-white text-xs font-bold rounded-lg hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 active:scale-95 disabled:opacity-50 disabled:cursor-not-allowed"
                  >
                    {isSavingPlan ? "Saving..." : "Apply Plan"}
                  </button>
                </div>
              </section>

              {/* Billing History Card */}
              <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden shadow-sm">
                <div className="p-6 border-b border-slate-200 dark:border-slate-800 flex justify-between items-center">
                  <div>
                    <h3 className="text-lg font-bold">Billing History</h3>
                    <p className="text-sm text-slate-500">Past invoices and payment status.</p>
                  </div>
                  <button
                    disabled
                    className="text-xs font-bold text-slate-300 cursor-not-allowed px-2 py-1"
                  >
                    Download All
                  </button>
                </div>
                <div className="overflow-x-auto">
                  <table className="w-full text-left">
                    <thead>
                      <tr className="text-[10px] font-bold text-slate-400 uppercase tracking-widest bg-slate-50/50 dark:bg-slate-800/30">
                        <th className="py-3 px-6">Date</th>
                        <th className="py-3 px-6">Amount</th>
                        <th className="py-3 px-6">Status</th>
                        <th className="py-3 px-6 text-right">Invoice</th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 dark:divide-slate-800">
                      {billingHistory.length === 0 ? (
                        <tr>
                          <td
                            className="py-6 px-6 text-xs text-slate-500"
                            colSpan={4}
                          >
                            No billing records yet.
                          </td>
                        </tr>
                      ) : (
                        billingHistory.map((item) => (
                          <tr
                            key={item.id}
                            className="text-sm font-medium hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors"
                          >
                            <td className="py-4 px-6 text-slate-700 dark:text-slate-300 text-xs font-bold">
                              {formatDate(item.issued_at)}
                            </td>
                            <td className="py-4 px-6 text-slate-900 dark:text-white font-black text-xs">
                              {item.amount}
                            </td>
                            <td className="py-4 px-6">
                              <span
                                className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold ${billingStatusStyles(
                                  item.status,
                                )}`}
                              >
                                <div
                                  className={`size-1.5 rounded-full ${
                                    item.status === "paid"
                                      ? "bg-emerald-500"
                                      : item.status === "pending"
                                        ? "bg-amber-500"
                                        : "bg-rose-500"
                                  }`}
                                />
                                {item.status.toUpperCase()}
                              </span>
                            </td>
                            <td className="py-4 px-6 text-right">
                              <button
                                onClick={() => handleDownloadReceipt(item.id)}
                                className="p-2 text-slate-400 hover:text-primary hover:bg-primary/5 rounded-lg transition-all"
                                aria-label="Download invoice"
                              >
                                <Download size={18} />
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </section>
            </div>

            {/* Right Column: Payment & Support */}
            <div className="space-y-6">
              {/* Payment Method Card */}
              <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 p-6 shadow-sm">
                <div className="flex justify-between items-center mb-6">
                  <h3 className="text-md font-bold text-slate-900 dark:text-white">
                    Payment Method
                  </h3>
                  <button className="text-xs font-bold text-slate-400 cursor-not-allowed">
                    Update
                  </button>
                </div>
                <div className="relative group">
                  <div className="absolute inset-0 bg-primary/5 rounded-lg blur transition-opacity opacity-0 group-hover:opacity-100" />
                  <div className="relative p-4 rounded-lg border border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 flex items-center gap-4">
                    <div className="size-10 rounded-lg bg-slate-100 dark:bg-slate-800 flex items-center justify-center border border-slate-200 dark:border-slate-700">
                      <CreditCard size={20} className="text-slate-500" />
                    </div>
                    <div className="flex-1 min-w-0">
                      <p className="text-xs font-bold text-slate-900 dark:text-white truncate">
                        {`${(paymentMethod.brand || "card").toUpperCase()} ending in ${paymentMethod.last4}`}
                      </p>
                      <p className="text-[10px] text-slate-500 dark:text-slate-400">
                        Expires {String(paymentMethod.exp_month).padStart(2, "0")}/
                        {String(paymentMethod.exp_year).slice(-2)}
                      </p>
                    </div>
                  </div>
                </div>
                <p className="text-[10px] text-slate-400 mt-4 text-center">
                  Billing profile stored by backend service
                </p>
              </section>

              {/* Support/Quick Actions */}
              <section className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-lg shadow-slate-900/10 border border-slate-800">
                <h4 className="font-bold mb-2">Need help?</h4>
                <p className="text-xs text-slate-400 mb-6 leading-relaxed">
                  Contact support if your invoice or subscription status looks incorrect.
                </p>
                <div className="space-y-2">
                  <button className="w-full py-2 bg-white/10 hover:bg-white/20 rounded-lg text-xs font-bold transition-colors">
                    View FAQ
                  </button>
                  <button className="w-full py-2 bg-white text-slate-900 hover:bg-slate-100 rounded-lg text-xs font-bold transition-colors shadow-sm">
                    Contact Support
                  </button>
                </div>
              </section>

              {/* Danger Zone */}
              <div className="pt-2">
                <button
                  onClick={handleCancelSubscription}
                  disabled={isCancelling || isSavingPlan || subscription.plan === "free"}
                  className="w-full flex items-center justify-center gap-2 py-3 text-[10px] font-bold text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-900/10 rounded-lg transition-colors border border-transparent hover:border-rose-100 dark:hover:border-rose-900/50 disabled:opacity-40 disabled:cursor-not-allowed"
                >
                  <XCircle size={14} weight="fill" />
                  {isCancelling
                    ? "UPDATING..."
                    : isCancelScheduled
                      ? "REACTIVATE SUBSCRIPTION"
                      : "CANCEL AT PERIOD END"}
                </button>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
