"use client";

import {
  TrayArrowDown,
  Gauge,
  ClockCounterClockwise,
  Gear,
  SignOut,
} from "@phosphor-icons/react";
import type { UserProfile } from "@/data/settings-data";

interface SettingsSidebarProps {
  user: UserProfile;
  onLogout: () => void;
}

export default function SettingsSidebar({
  user,
  onLogout,
}: SettingsSidebarProps) {
  const navItems = [
    { icon: Gauge, label: "Dashboard", href: "/", active: false },
    {
      icon: ClockCounterClockwise,
      label: "History",
      href: "/history",
      active: false,
    },
    { icon: Gear, label: "Settings", href: "/settings", active: true },
  ];

  return (
    <aside className="w-64 border-r border-slate-200 dark:border-slate-800 bg-white dark:bg-slate-900 flex flex-col">
      <div className="p-6 flex items-center gap-3">
        <div className="size-8 bg-primary rounded-lg flex items-center justify-center text-white">
          <TrayArrowDown size={20} weight="fill" />
        </div>
        <h2 className="text-xl font-bold tracking-tight">QuickSnap</h2>
      </div>
      <nav className="flex-1 px-4 space-y-1">
        {navItems.map((item) => (
          <a
            key={item.label}
            className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-colors ${
              item.active
                ? "bg-primary/10 text-primary"
                : "text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
            }`}
            href={item.href}
          >
            <item.icon
              size={20}
              weight={item.active ? "fill" : "regular"}
              className={item.active ? "text-primary" : ""}
            />
            <span className="font-medium text-sm">{item.label}</span>
          </a>
        ))}
      </nav>
      <div className="p-4 border-t border-slate-200 dark:border-slate-800">
        <div className="flex items-center gap-3 px-2 py-2">
          <div className="size-8 rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              className="w-full h-full object-cover"
              src={user.avatar}
              alt={user.name}
            />
          </div>
          <div className="flex-1 min-w-0">
            <p className="text-xs font-bold truncate">{user.name}</p>
            <p className="text-[10px] text-slate-500 truncate">{user.plan}</p>
          </div>
          <button
            onClick={onLogout}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
            aria-label="Logout"
          >
            <SignOut size={20} />
          </button>
        </div>
      </div>
    </aside>
  );
}
