"use client";

import { Envelope } from "@phosphor-icons/react";
import type { NotificationPreference } from "@/data/settings-data";

interface SettingsNotificationsProps {
  emailAlerts: NotificationPreference[];
  onAlertChange: (id: string, checked: boolean) => void;
}

export default function SettingsNotifications({
  emailAlerts,
  onAlertChange,
}: SettingsNotificationsProps) {
  return (
    <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden">
      <div className="p-6 border-b border-slate-200 dark:border-slate-800">
        <h3 className="text-lg font-bold">Notifications</h3>
        <p className="text-sm text-slate-500">
          Control when and how you receive status updates.
        </p>
      </div>
      <div className="p-6 space-y-4">
        <div className="flex items-start gap-4 p-4 rounded-lg bg-slate-50 dark:bg-slate-800/50">
          <div className="p-2 bg-primary/10 rounded-lg">
            <Envelope size={20} weight="fill" className="text-primary" />
          </div>
          <div className="flex-1 space-y-3">
            <p className="text-sm font-bold">Email Alerts</p>
            <div className="space-y-3">
              {emailAlerts.map((alert) => (
                <label
                  key={alert.id}
                  className="flex items-center gap-3 cursor-pointer group"
                >
                  <input
                    type="checkbox"
                    className="w-4 h-4 rounded border-slate-300 text-primary focus:ring-primary"
                    checked={alert.checked}
                    onChange={(e) => onAlertChange(alert.id, e.target.checked)}
                  />
                  <span className="text-sm text-slate-600 dark:text-slate-400 group-hover:text-slate-900 dark:group-hover:text-slate-200">
                    {alert.label}
                  </span>
                </label>
              ))}
            </div>
          </div>
        </div>
      </div>
    </section>
  );
}
