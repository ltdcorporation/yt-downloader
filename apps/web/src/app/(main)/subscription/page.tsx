"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError } from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { CreditCard, Download, CheckCircle, XCircle } from "@phosphor-icons/react";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";

interface BillingHistoryItem {
  id: string;
  date: string;
  amount: string;
  status: "paid" | "pending" | "failed";
  receiptUrl?: string;
}

interface PlanBenefit {
  id: string;
  label: string;
}

const MOCK_PLAN_BENEFITS: PlanBenefit[] = [
  { id: "1", label: "Unlimited 4K Downloads" },
  { id: "2", label: "Priority Processing" },
  { id: "3", label: "Cloud Storage (50GB)" },
  { id: "4", label: "Auto-Trim & Silence Removal" },
];

const MOCK_BILLING_HISTORY: BillingHistoryItem[] = [
  {
    id: "1",
    date: "Sep 12, 2024",
    amount: "$12.00",
    status: "paid",
  },
  {
    id: "2",
    date: "Aug 12, 2024",
    amount: "$12.00",
    status: "paid",
  },
  {
    id: "3",
    date: "Jul 12, 2024",
    amount: "$12.00",
    status: "paid",
  },
];

export default function SubscriptionPage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [loadError, setLoadError] = useState("");

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

  const handleDownloadReceipt = (_id: string) => {
    // TODO: Implement receipt download when backend API is ready
    console.log("Download receipt:", _id);
  };

  const handleManagePlan = () => {
    // TODO: Implement plan management when backend API is ready
    console.log("Manage plan");
  };

  const handleCancelSubscription = () => {
    // TODO: Implement subscription cancellation when backend API is ready
    console.log("Cancel subscription");
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

  const isAdmin = currentUser.email === "admin@example.com";

  const userProfile = {
    name: currentUser.full_name,
    email: currentUser.email,
    plan: isAdmin ? "Super Admin" : "Pro Plan",
    avatar: currentUser.avatar_url || DEFAULT_AVATAR_URL,
  };

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
            <div className="flex items-center gap-2 px-3 py-1.5 bg-emerald-50 dark:bg-emerald-900/20 border border-emerald-100 dark:border-emerald-800/50 rounded-full">
              <div className="size-2 rounded-full bg-emerald-500 animate-pulse" />
              <span className="text-[10px] font-bold text-emerald-700 dark:text-emerald-400 tracking-wider">
                PRO PLAN ACTIVE
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
                <div className="p-6">
                  <div className="flex justify-between items-start mb-8">
                    <div className="flex items-center gap-4">
                      <div className="size-12 rounded-xl bg-primary/10 flex items-center justify-center text-primary">
                        <CreditCard size={24} weight="fill" />
                      </div>
                      <div>
                        <h2 className="text-lg font-bold text-slate-900 dark:text-white">
                          QuickSnap Pro
                        </h2>
                        <p className="text-sm font-medium text-slate-500 dark:text-slate-400">
                          Next billing: Oct 12, 2024
                        </p>
                      </div>
                    </div>
                    <div className="text-right">
                      <p className="text-xl font-black text-slate-900 dark:text-white">
                        $12.00
                      </p>
                      <p className="text-[10px] font-bold text-slate-400 uppercase tracking-wider">per month</p>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                    {MOCK_PLAN_BENEFITS.map((benefit) => (
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
                <div className="px-6 py-4 bg-slate-50 dark:bg-slate-800/50 border-t border-slate-200 dark:border-slate-800 flex justify-between items-center">
                  <p className="text-xs text-slate-500 font-medium italic">
                    You&apos;re saving 15% with your current plan.
                  </p>
                  <button
                    onClick={handleManagePlan}
                    className="px-4 py-2 bg-primary text-white text-xs font-bold rounded-lg hover:bg-primary/90 transition-all shadow-lg shadow-primary/20 active:scale-95"
                  >
                    Manage Subscription
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
                  <button className="text-xs font-bold text-primary hover:underline px-2 py-1">
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
                      {MOCK_BILLING_HISTORY.map((item) => (
                        <tr
                          key={item.id}
                          className="text-sm font-medium hover:bg-slate-50/50 dark:hover:bg-slate-800/30 transition-colors"
                        >
                          <td className="py-4 px-6 text-slate-700 dark:text-slate-300 text-xs font-bold">
                            {item.date}
                          </td>
                          <td className="py-4 px-6 text-slate-900 dark:text-white font-black text-xs">
                            {item.amount}
                          </td>
                          <td className="py-4 px-6">
                            <span
                              className={`inline-flex items-center gap-1.5 px-2.5 py-1 rounded-full text-[10px] font-bold ${
                                item.status === "paid"
                                  ? "bg-emerald-100 text-emerald-700 dark:bg-emerald-900/30 dark:text-emerald-400"
                                  : "bg-rose-100 text-rose-700 dark:bg-rose-900/30 dark:text-rose-400"
                              }`}
                            >
                              <div
                                className={`size-1.5 rounded-full ${
                                  item.status === "paid"
                                    ? "bg-emerald-500"
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
                      ))}
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
                  <button className="text-xs font-bold text-primary hover:underline">
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
                        Visa ending in 4242
                      </p>
                      <p className="text-[10px] text-slate-500 dark:text-slate-400">
                        Expires 12/26
                      </p>
                    </div>
                  </div>
                </div>
                <p className="text-[10px] text-slate-400 mt-4 text-center">
                  Secure payments powered by Stripe
                </p>
              </section>

              {/* Support/Quick Actions */}
              <section className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-lg shadow-slate-900/10 border border-slate-800">
                <h4 className="font-bold mb-2">Need help?</h4>
                <p className="text-xs text-slate-400 mb-6 leading-relaxed">
                  Check our documentation or contact support for billing issues.
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
                  className="w-full flex items-center justify-center gap-2 py-3 text-[10px] font-bold text-rose-500 hover:bg-rose-50 dark:hover:bg-rose-900/10 rounded-lg transition-colors border border-transparent hover:border-rose-100 dark:hover:border-rose-900/50"
                >
                  <XCircle size={14} weight="fill" />
                  CANCEL SUBSCRIPTION
                </button>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
