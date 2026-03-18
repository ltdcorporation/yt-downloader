"use client";

import { Bell, Question } from "@phosphor-icons/react";

export default function SettingsHeader() {
  return (
    <header className="sticky top-0 z-10 bg-background-light/80 dark:bg-background-dark/80 backdrop-blur-md px-8 py-6 flex justify-between items-center">
      <div>
        <h1 className="text-2xl font-black tracking-tight">Settings</h1>
        <p className="text-slate-500 text-sm">
          Manage your account and app preferences.
        </p>
      </div>
      <div className="flex items-center gap-3">
        <button
          className="p-2 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:text-primary hover:border-primary transition-colors"
          aria-label="Notifications"
        >
          <Bell size={20} />
        </button>
        <button
          className="p-2 rounded-full bg-white dark:bg-slate-800 border border-slate-200 dark:border-slate-700 text-slate-600 dark:text-slate-400 hover:text-primary hover:border-primary transition-colors"
          aria-label="Help"
        >
          <Question size={20} />
        </button>
      </div>
    </header>
  );
}
