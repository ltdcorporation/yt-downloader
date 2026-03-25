"use client";

import {
  TrayArrowDown,
  Gauge,
  ClockCounterClockwise,
  Gear,
  SignOut,
  CreditCard,
  List,
  CaretLeft,
  X,
  ArrowLeft,
  ShieldCheck,
  ChartBar,
  Users,
} from "@phosphor-icons/react";
import type { UserProfile } from "@/data/settings-data";
import { useState, useEffect } from "react";
import { usePathname } from "next/navigation";
import Link from "next/link";

interface NavItem {
  icon: React.ElementType;
  label: string;
  href: string;
  active: boolean;
}

interface SettingsSidebarProps {
  user: UserProfile;
  onLogout: () => void;
  isOpen?: boolean;
  onClose?: () => void;
  navItems?: NavItem[];
}

export default function SettingsSidebar({
  user,
  onLogout,
  isOpen = false,
  onClose,
  navItems: customNavItems,
}: SettingsSidebarProps) {
  const pathname = usePathname();
  const [isCollapsed, setIsCollapsed] = useState(false);

  const defaultNavItems = [
    {
      icon: CreditCard,
      label: "Subscription",
      href: "/subscription",
      active: pathname === "/subscription",
    },
    {
      icon: Gear,
      label: "Settings",
      href: "/settings",
      active: pathname === "/settings",
    },
    {
      icon: ShieldCheck,
      label: "Admin",
      href: "/admin",
      active: pathname.startsWith("/admin"),
    },
  ];

  const items = customNavItems || defaultNavItems;

  return (
    <>
      {/* Mobile Overlay */}
      {isOpen && (
        <div
          className="fixed inset-0 z-40 bg-slate-900/50 backdrop-blur-sm lg:hidden"
          onClick={onClose}
        />
      )}

      <aside
        className={`
          fixed inset-y-0 left-0 z-50 flex flex-col bg-white dark:bg-slate-900 border-r border-slate-200 dark:border-slate-800 transition-all duration-300 ease-in-out
          ${isOpen ? "translate-x-0" : "-translate-x-full lg:translate-x-0"}
          ${isCollapsed ? "lg:w-20" : "w-64"}
          lg:relative
        `}
      >
        <div className={`p-6 flex items-center justify-between gap-3 ${isCollapsed ? "lg:px-0 lg:justify-center" : ""}`}>
          <div className="flex items-center gap-3 overflow-hidden">
            <div className="shrink-0 size-8 bg-primary rounded-lg flex items-center justify-center text-white">
              <TrayArrowDown size={20} weight="fill" />
            </div>
            {!isCollapsed && (
              <h2 className="text-xl font-bold tracking-tight truncate whitespace-nowrap">
                QuickSnap
              </h2>
            )}
          </div>
          
          {/* Collapse Toggle Button (Desktop) */}
          <button
            onClick={() => setIsCollapsed(!isCollapsed)}
            className="hidden lg:flex p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500 transition-colors"
            title={isCollapsed ? "Expand Sidebar" : "Collapse Sidebar"}
          >
            <CaretLeft size={18} className={`transition-transform duration-300 ${isCollapsed ? "rotate-180" : ""}`} />
          </button>

          {/* Close Button (Mobile) */}
          <button
            onClick={onClose}
            className="lg:hidden p-1.5 rounded-lg hover:bg-slate-100 dark:hover:bg-slate-800 text-slate-500"
          >
            <X size={20} />
          </button>
        </div>

        <nav className={`flex-1 px-4 space-y-1 ${isCollapsed ? "lg:px-2" : ""}`}>
          {items.map((item) => (
            <a
              key={item.label}
              className={`flex items-center gap-3 px-3 py-2 rounded-lg transition-colors group ${
                isCollapsed ? "lg:justify-center" : ""
              } ${
                item.active
                  ? "bg-primary/10 text-primary"
                  : "text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800"
              }`}
              href={item.href}
              title={isCollapsed ? item.label : ""}
            >
              <item.icon
                size={20}
                weight={item.active ? "fill" : "regular"}
                className={`${item.active ? "text-primary" : ""} shrink-0`}
              />
              {!isCollapsed && (
                <span className="font-medium text-sm truncate whitespace-nowrap">
                  {item.label}
                </span>
              )}
              {isCollapsed && (
                <span className="lg:hidden font-medium text-sm truncate whitespace-nowrap">
                  {item.label}
                </span>
              )}
            </a>
          ))}
        </nav>

        <div className={`p-4 border-t border-slate-200 dark:border-slate-800 ${isCollapsed ? "lg:p-2" : ""}`}>
          <Link
            href="/"
            className={`flex items-center gap-3 px-3 py-2 mb-2 rounded-lg text-slate-600 dark:text-slate-400 hover:bg-slate-50 dark:hover:bg-slate-800 transition-colors group ${
              isCollapsed ? "lg:justify-center" : ""
            }`}
            title={isCollapsed ? "Back to Home" : ""}
          >
            <ArrowLeft size={20} className="shrink-0" />
            {!isCollapsed && (
              <span className="font-medium text-sm truncate whitespace-nowrap">
                Back to Home
              </span>
            )}
          </Link>
          <div className={`flex items-center gap-3 px-2 py-2 ${isCollapsed ? "lg:justify-center lg:px-0" : ""}`}>
            <div className="shrink-0 size-8 rounded-full bg-slate-200 dark:bg-slate-700 overflow-hidden">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                className="w-full h-full object-cover"
                src={user.avatar}
                alt={user.name}
              />
            </div>
            {!isCollapsed && (
              <div className="flex-1 min-w-0">
                <p className="text-xs font-bold truncate">{user.name}</p>
                <p className="text-[10px] text-slate-500 truncate">{user.plan}</p>
              </div>
            )}
            {!isCollapsed && (
              <button
                onClick={onLogout}
                className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors shrink-0"
                aria-label="Logout"
              >
                <SignOut size={20} />
              </button>
            )}
          </div>
          {isCollapsed && (
            <button
              onClick={onLogout}
              className="hidden lg:flex w-full items-center justify-center py-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-300 transition-colors"
              aria-label="Logout"
            >
              <SignOut size={20} />
            </button>
          )}
        </div>
      </aside>
    </>
  );
}
