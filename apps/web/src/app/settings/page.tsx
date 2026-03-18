"use client";

import { useState } from "react";
import SettingsSidebar from "@/components/settings/SettingsSidebar";
import SettingsHeader from "@/components/settings/SettingsHeader";
import SettingsProfile from "@/components/settings/SettingsProfile";
import SettingsPreferences from "@/components/settings/SettingsPreferences";
import SettingsNotifications from "@/components/settings/SettingsNotifications";
import {
  SAMPLE_USER,
  DEFAULT_SETTINGS,
  type SettingsFormData,
} from "@/data/settings-data";

export default function SettingsPage() {
  const [formData, setFormData] = useState<SettingsFormData>(DEFAULT_SETTINGS);

  const handleFullNameChange = (value: string) => {
    setFormData((prev) => ({ ...prev, fullName: value }));
  };

  const handleEmailChange = (value: string) => {
    setFormData((prev) => ({ ...prev, email: value }));
  };

  const handleQualityChange = (value: SettingsFormData["defaultQuality"]) => {
    setFormData((prev) => ({ ...prev, defaultQuality: value }));
  };

  const handleAutoTrimChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, autoTrimSilence: value }));
  };

  const handleThumbnailChange = (value: boolean) => {
    setFormData((prev) => ({ ...prev, thumbnailGeneration: value }));
  };

  const handleAlertChange = (id: string, checked: boolean) => {
    setFormData((prev) => ({
      ...prev,
      emailAlerts: prev.emailAlerts.map((alert) =>
        alert.id === id ? { ...alert, checked } : alert
      ),
    }));
  };

  const handleSave = () => {
    console.log("Saving settings:", formData);
    // TODO: Implement API call to save settings
  };

  const handleDiscard = () => {
    setFormData(DEFAULT_SETTINGS);
  };

  const handleLogout = () => {
    console.log("Logout clicked");
    // TODO: Implement logout logic
  };

  return (
    <div className="flex h-screen overflow-hidden">
      <SettingsSidebar user={SAMPLE_USER} onLogout={handleLogout} />
      <main className="flex-1 overflow-y-auto bg-background-light dark:bg-background-dark">
        <SettingsHeader />
        <div className="max-w-4xl px-8 pb-12 space-y-8">
          <SettingsProfile
            user={SAMPLE_USER}
            fullName={formData.fullName}
            email={formData.email}
            onFullNameChange={handleFullNameChange}
            onEmailChange={handleEmailChange}
          />
          <SettingsPreferences
            defaultQuality={formData.defaultQuality}
            autoTrimSilence={formData.autoTrimSilence}
            thumbnailGeneration={formData.thumbnailGeneration}
            onQualityChange={handleQualityChange}
            onAutoTrimChange={handleAutoTrimChange}
            onThumbnailChange={handleThumbnailChange}
          />
          <SettingsNotifications
            emailAlerts={formData.emailAlerts}
            onAlertChange={handleAlertChange}
          />
          <div className="flex items-center justify-end gap-3 pt-4">
            <button
              onClick={handleDiscard}
              className="px-6 py-2.5 text-sm font-bold text-slate-600 dark:text-slate-400 hover:text-slate-900 dark:hover:text-slate-100 transition-colors"
            >
              Discard Changes
            </button>
            <button
              onClick={handleSave}
              className="px-8 py-2.5 bg-primary text-white text-sm font-bold rounded-lg shadow-lg shadow-primary/20 hover:bg-primary/90 transition-all"
            >
              Save Preferences
            </button>
          </div>
        </div>
      </main>
    </div>
  );
}
