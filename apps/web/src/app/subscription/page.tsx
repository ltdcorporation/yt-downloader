"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuthStore } from "@/store";
import { api, APIError } from "@/lib/api";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import { CreditCard, Download, CheckCircle, XCircle } from "@phosphor-icons/react";

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

  const userProfile = {
    name: currentUser.full_name,
    email: currentUser.email,
    plan: "Free Plan",
    avatar: currentUser.avatar_url || "/default-avatar.png",
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
        <SettingsHeader onMenuClick={() => setIsSidebarOpen(true)} />
        <div className="max-w-4xl px-8 pb-12 space-y-6">
          {loadError ? (
            <div className="rounded-lg border border-rose-200 bg-rose-50 px-4 py-3 text-sm text-rose-700 dark:border-rose-900/60 dark:bg-rose-950/30 dark:text-rose-300">
              {loadError}
            </div>
          ) : null}

          {/* Page Header */}
          <div className="mb-8">
            <h1 className="text-2xl font-black tracking-tight text-on-surface mb-2">
              Subscription Management
            </h1>
            <p className="text-sm text-on-surface-variant font-medium">
              Manage your billing information and plan details below.
            </p>
          </div>

          {/* Main Subscription Card */}
          <div className="bg-surface rounded-lg shadow-lg shadow-primary/5 border border-outline overflow-hidden">
            {/* 1. Current Plan Section */}
            <div className="p-8 border-b border-outline">
              <div className="flex flex-col md:flex-row justify-between items-start md:items-center gap-6">
                <div className="flex items-center gap-4">
                  <div className="w-12 h-12 rounded-lg bg-primary-container flex items-center justify-center text-primary">
                    <CreditCard size={24} weight="fill" />
                  </div>
                  <div>
                    <div className="flex items-center gap-2">
                      <h2 className="text-lg font-bold text-on-surface">
                        QuickSnap Pro
                      </h2>
                      <span className="bg-primary/10 text-primary text-[10px] font-bold px-2 py-0.5 rounded-full uppercase tracking-wider">
                        Active
                      </span>
                    </div>
                    <p className="text-sm font-medium text-on-surface-variant">
                      $12.00/month • Next Billing Date: October 12, 2024
                    </p>
                  </div>
                </div>
                <button
                  onClick={handleManagePlan}
                  className="bg-primary text-on-primary px-5 py-2.5 rounded text-sm font-bold shadow-lg shadow-primary/20 active:scale-95 transition-transform"
                >
                  Manage Plan
                </button>
              </div>
            </div>

            {/* 2. Plan Benefits Section */}
            <div className="p-8 border-b border-outline bg-surface-container-low">
              <h3 className="text-xs font-bold text-on-surface-variant uppercase tracking-widest mb-4">
                Included in your plan
              </h3>
              <div className="grid grid-cols-1 md:grid-cols-2 gap-y-4 gap-x-12">
                {MOCK_PLAN_BENEFITS.map((benefit) => (
                  <div key={benefit.id} className="flex items-center gap-3">
                    <CheckCircle
                      size={20}
                      weight="fill"
                      className="text-primary shrink-0"
                    />
                    <span className="text-sm font-medium text-on-surface">
                      {benefit.label}
                    </span>
                  </div>
                ))}
              </div>
            </div>

            {/* 3. Payment Methods */}
            <div className="p-8 border-b border-outline">
              <div className="flex justify-between items-center mb-6">
                <h3 className="text-xs font-bold text-on-surface-variant uppercase tracking-widest">
                  Payment Method
                </h3>
                <button className="text-xs font-bold text-primary hover:underline">
                  Edit
                </button>
              </div>
              <div className="bg-surface-container rounded-lg p-5 flex items-center justify-between border border-outline">
                <div className="flex items-center gap-4">
                  <div className="bg-white p-2 border border-outline-variant rounded shadow-sm">
                    <CreditCard size={24} className="text-on-surface-variant" />
                  </div>
                  <div>
                    <p className="text-sm font-bold text-on-surface">
                      Visa ending in 4242
                    </p>
                    <p className="text-xs text-on-surface-variant">
                      Expires 12/26
                    </p>
                  </div>
                </div>
                <div className="hidden sm:block">
                  <span className="text-[10px] font-bold text-secondary-fixed-dim bg-secondary-fixed px-2 py-1 rounded">
                    DEFAULT
                  </span>
                </div>
              </div>
            </div>

            {/* 4. Billing History */}
            <div className="p-8">
              <h3 className="text-xs font-bold text-on-surface-variant uppercase tracking-widest mb-6">
                Billing History
              </h3>
              <div className="overflow-x-auto">
                <table className="w-full text-left">
                  <thead>
                    <tr className="text-[10px] font-bold text-on-surface-variant border-b border-outline">
                      <th className="pb-3 px-2">DATE</th>
                      <th className="pb-3 px-2">AMOUNT</th>
                      <th className="pb-3 px-2">STATUS</th>
                      <th className="pb-3 px-2 text-right">RECEIPT</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-outline">
                    {MOCK_BILLING_HISTORY.map((item) => (
                      <tr key={item.id} className="text-sm font-medium">
                        <td className="py-4 px-2 text-on-surface">{item.date}</td>
                        <td className="py-4 px-2 text-on-surface">{item.amount}</td>
                        <td className="py-4 px-2">
                          <span
                            className={`text-[10px] font-bold px-2 py-0.5 rounded-full ${
                              item.status === "paid"
                                ? "bg-emerald-50 text-emerald-700 dark:bg-emerald-950/30 dark:text-emerald-400"
                                : item.status === "pending"
                                ? "bg-amber-50 text-amber-700 dark:bg-amber-950/30 dark:text-amber-400"
                                : "bg-rose-50 text-rose-700 dark:bg-rose-950/30 dark:text-rose-400"
                            }`}
                          >
                            {item.status.toUpperCase()}
                          </span>
                        </td>
                        <td className="py-4 px-2 text-right">
                          <button
                            onClick={() => handleDownloadReceipt(item.id)}
                            className="text-on-surface-variant hover:text-primary transition-colors"
                            aria-label="Download receipt"
                          >
                            <Download size={20} />
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </div>
          </div>

          {/* Danger Zone / Footer Action */}
          <div className="mt-8 flex justify-center">
            <button
              onClick={handleCancelSubscription}
              className="text-xs font-bold text-error hover:underline flex items-center gap-2"
            >
              <XCircle size={16} weight="fill" />
              Cancel Subscription
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
