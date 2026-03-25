"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter, usePathname, useParams } from "next/navigation";
import Image from "next/image";
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
  ArrowLeft,
  Calendar,
  Envelope,
  CreditCard,
  DownloadSimple,
  Clock,
  ShieldCheck,
  Prohibit,
  CheckCircle,
  PencilSimple,
} from "@phosphor-icons/react";

interface UserDetail {
  id: string;
  name: string;
  email: string;
  avatar?: string;
  joinDate: string;
  plan: "pro" | "free" | "suspended";
  downloads: number;
  lastActive: string;
  location: string;
  bio: string;
}

// Mock data fetch simulation
const getMockUser = (id: string): UserDetail => ({
  id,
  name: id === "1" ? "Sarah Jenkins" : id === "2" ? "Marcus Thorne" : "Elena Rodriguez",
  email: id === "1" ? "sarah.j@example.com" : id === "2" ? "m.thorne@quickmail.net" : "elena.rod@webflow.io",
  avatar: id === "1" ? "https://images.unsplash.com/photo-1494790108377-be9c29b29330?w=200&h=200&fit=crop" : 
          id === "2" ? "https://images.unsplash.com/photo-1507003211169-0a1dd7228f2d?w=200&h=200&fit=crop" : 
          "https://images.unsplash.com/photo-1438761681033-6461ffad8d80?w=200&h=200&fit=crop",
  joinDate: "Oct 24, 2023",
  plan: id === "4" ? "suspended" : id === "2" ? "free" : "pro",
  downloads: 1240,
  lastActive: "2 hours ago",
  location: "New York, USA",
  bio: "Digital content creator and frequent traveler. Loves high-quality video formats.",
});

export default function AdminUserDetailPage() {
  const { currentUser, logout, isAuthChecking, setCurrentUser, setIsAuthChecking } = useAuthStore();
  const router = useRouter();
  const pathname = usePathname();
  const params = useParams();
  const userId = params.id as string;

  const [isPageLoading, setIsPageLoading] = useState(true);
  const [isSidebarOpen, setIsSidebarOpen] = useState(false);
  const [user, setUser] = useState<UserDetail | null>(null);

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
    // Simulate loading user data
    setUser(getMockUser(userId));
    setIsPageLoading(false);
  }, [currentUser, isAuthChecking, router, userId]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch { /* noop */ }
    logout();
    router.push("/");
  };

  if (isAuthChecking || isPageLoading || !user) {
    return (
      <div className="flex h-screen items-center justify-center bg-background-light dark:bg-background-dark">
        <div className="animate-spin rounded-full h-8 w-8 border-b-2 border-primary"></div>
      </div>
    );
  }

  const userProfile = {
    name: currentUser!.full_name,
    email: currentUser!.email,
    plan: "Super Admin",
    avatar: currentUser!.avatar_url || DEFAULT_AVATAR_URL,
  };

  const adminNavItems = [
    { icon: Layout, label: "Dashboard", href: "/admin", active: false },
    { icon: Users, label: "Users", href: "/admin/users", active: true },
    { icon: Wrench, label: "Maintenance", href: "/admin/maintenance", active: false },
    { icon: Gear, label: "Settings", href: "/admin/settings", active: false },
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
          title="User Details"
          showText={false}
        />

        <div className="max-w-5xl mx-auto pt-4 px-4 sm:px-8 pb-12 space-y-6">
          {/* Back Button */}
          <button
            onClick={() => router.back()}
            className="flex items-center gap-2 text-slate-500 hover:text-primary transition-colors text-sm font-bold group"
          >
            <ArrowLeft size={18} className="group-hover:-translate-x-1 transition-transform" />
            Back to User List
          </button>

          {/* User Profile Header Card */}
          <section className="bg-white dark:bg-slate-900 rounded-xl p-8 border border-slate-200 dark:border-slate-800 shadow-sm">
            <div className="flex flex-col md:flex-row gap-8 items-start">
              <div className="relative group">
                <Image
                  src={user.avatar || DEFAULT_AVATAR_URL}
                  alt={user.name}
                  width={128}
                  height={128}
                  className="size-32 rounded-2xl object-cover border-4 border-slate-50 dark:border-slate-800 shadow-xl"
                />
                <div className={`absolute -bottom-2 -right-2 size-6 rounded-full border-4 border-white dark:border-slate-900 ${
                  user.plan === 'suspended' ? 'bg-rose-500' : 'bg-emerald-500'
                }`} />
              </div>

              <div className="flex-1 space-y-4">
                <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4">
                  <div>
                    <h1 className="text-3xl font-black text-slate-900 dark:text-white tracking-tight">
                      {user.name}
                    </h1>
                    <p className="text-slate-500 font-medium">{user.email}</p>
                  </div>
                  <div className="flex gap-2">
                    <button className="px-4 py-2 bg-slate-100 dark:bg-slate-800 text-slate-900 dark:text-white font-bold text-xs rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 transition-all flex items-center gap-2">
                      <PencilSimple size={16} />
                      Edit Profile
                    </button>
                    {user.plan === 'suspended' ? (
                      <button className="px-4 py-2 bg-emerald-500 text-white font-bold text-xs rounded-lg hover:bg-emerald-600 transition-all flex items-center gap-2 shadow-lg shadow-emerald-500/20">
                        <CheckCircle size={16} />
                        Unsuspend
                      </button>
                    ) : (
                      <button className="px-4 py-2 bg-rose-500 text-white font-bold text-xs rounded-lg hover:bg-rose-600 transition-all flex items-center gap-2 shadow-lg shadow-rose-500/20">
                        <Prohibit size={16} />
                        Suspend User
                      </button>
                    )}
                  </div>
                </div>

                <div className="grid grid-cols-2 md:grid-cols-4 gap-4 pt-4 border-t border-slate-100 dark:border-slate-800">
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">Plan</p>
                    <span className={`inline-block px-2 py-0.5 rounded text-[10px] font-black uppercase ${
                      user.plan === 'pro' ? 'bg-primary/10 text-primary' : 
                      user.plan === 'suspended' ? 'bg-rose-100 text-rose-600' : 'bg-slate-100 text-slate-500'
                    }`}>
                      {user.plan}
                    </span>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">Total Downloads</p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">{user.downloads.toLocaleString()}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">Joined</p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">{user.joinDate}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[10px] font-black text-slate-400 uppercase tracking-widest">Last Active</p>
                    <p className="text-sm font-bold text-slate-900 dark:text-white">{user.lastActive}</p>
                  </div>
                </div>
              </div>
            </div>
          </section>

          {/* Details Grid */}
          <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
            <div className="md:col-span-2 space-y-6">
              {/* Activity Log */}
              <div className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden">
                <div className="p-6 border-b border-slate-100 dark:border-slate-800">
                  <h3 className="font-bold flex items-center gap-2">
                    <Clock size={20} className="text-primary" />
                    Recent Activity
                  </h3>
                </div>
                <div className="divide-y divide-slate-50 dark:divide-slate-800">
                  {[1, 2, 3].map((i) => (
                    <div key={i} className="p-4 flex items-center justify-between hover:bg-slate-50 dark:hover:bg-slate-800/50 transition-colors">
                      <div className="flex items-center gap-3">
                        <div className="size-8 rounded-lg bg-emerald-50 dark:bg-emerald-900/20 flex items-center justify-center text-emerald-600">
                          <DownloadSimple size={18} />
                        </div>
                        <div>
                          <p className="text-sm font-bold text-slate-900 dark:text-white">YouTube Video Downloaded</p>
                          <p className="text-xs text-slate-500 font-medium">Video title example name {i}</p>
                        </div>
                      </div>
                      <p className="text-[10px] font-bold text-slate-400">12 mins ago</p>
                    </div>
                  ))}
                </div>
                <button className="w-full py-3 text-xs font-bold text-slate-500 hover:text-primary transition-colors bg-slate-50/50 dark:bg-slate-800/30">
                  View Full Logs
                </button>
              </div>
            </div>

            <div className="space-y-6">
              {/* Account Info */}
              <div className="bg-white dark:bg-slate-900 rounded-xl p-6 border border-slate-200 dark:border-slate-800 shadow-sm space-y-6">
                <h3 className="font-bold text-sm uppercase tracking-widest text-slate-400">Account Security</h3>
                
                <div className="space-y-4">
                  <div className="flex items-center gap-3">
                    <div className="size-10 rounded-full bg-blue-50 dark:bg-blue-900/20 flex items-center justify-center text-blue-600">
                      <ShieldCheck size={20} />
                    </div>
                    <div>
                      <p className="text-xs font-black text-slate-900 dark:text-white uppercase">Email Verified</p>
                      <p className="text-[10px] text-slate-500 font-medium">Jan 18, 2024</p>
                    </div>
                  </div>
                  <div className="flex items-center gap-3">
                    <div className="size-10 rounded-full bg-purple-50 dark:bg-blue-900/20 flex items-center justify-center text-purple-600">
                      <CreditCard size={20} />
                    </div>
                    <div>
                      <p className="text-xs font-black text-slate-900 dark:text-white uppercase">Billing Status</p>
                      <p className="text-[10px] text-slate-500 font-medium">Valid Payment Method</p>
                    </div>
                  </div>
                </div>
              </div>

              {/* Bio/Info */}
              <div className="bg-slate-900 dark:bg-slate-800 rounded-xl p-6 text-white shadow-xl relative overflow-hidden group">
                <Users size={80} weight="thin" className="absolute -bottom-4 -right-4 text-white/5 group-hover:scale-110 transition-transform duration-700" />
                <h4 className="font-bold text-sm mb-3 flex items-center gap-2 text-primary-light">
                  <Calendar size={18} />
                  Administrative Note
                </h4>
                <p className="text-slate-400 text-xs leading-relaxed italic">
                  &quot;{user.bio}&quot;
                </p>
              </div>
            </div>
          </div>
        </div>
      </main>
    </div>
  );
}
