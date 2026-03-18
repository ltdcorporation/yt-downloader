"use client";

import type { SettingsFormData } from "@/data/settings-data";
import { QUALITY_OPTIONS } from "@/data/settings-data";

interface SettingsPreferencesProps {
  defaultQuality: SettingsFormData["defaultQuality"];
  autoTrimSilence: boolean;
  thumbnailGeneration: boolean;
  onQualityChange: (value: SettingsFormData["defaultQuality"]) => void;
  onAutoTrimChange: (value: boolean) => void;
  onThumbnailChange: (value: boolean) => void;
}

export default function SettingsPreferences({
  defaultQuality,
  autoTrimSilence,
  thumbnailGeneration,
  onQualityChange,
  onAutoTrimChange,
  onThumbnailChange,
}: SettingsPreferencesProps) {
  return (
    <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden">
      <div className="p-6 border-b border-slate-200 dark:border-slate-800">
        <h3 className="text-lg font-bold">General Preferences</h3>
        <p className="text-sm text-slate-500">
          Configure your default download and processing settings.
        </p>
      </div>
      <div className="p-6 space-y-6">
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <p className="text-sm font-bold">Default Download Quality</p>
            <p className="text-xs text-slate-500">
              Set the preferred resolution for new video downloads.
            </p>
          </div>
          <select
            className="bg-slate-50 dark:bg-slate-800 border-none rounded-lg text-sm font-medium focus:ring-2 focus:ring-primary/50 py-2 pl-4 pr-10"
            value={defaultQuality}
            onChange={(e) =>
              onQualityChange(e.target.value as SettingsFormData["defaultQuality"])
            }
          >
            {QUALITY_OPTIONS.map((option) => (
              <option key={option.value} value={option.value}>
                {option.label}
              </option>
            ))}
          </select>
        </div>
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <p className="text-sm font-bold">Auto-Trim Silence</p>
            <p className="text-xs text-slate-500">
              Automatically remove silent parts from downloaded videos.
            </p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              className="sr-only peer"
              checked={autoTrimSilence}
              onChange={(e) => onAutoTrimChange(e.target.checked)}
            />
            <div className="w-11 h-6 bg-slate-200 peer-focus:outline-none dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary rounded-full" />
          </label>
        </div>
        <div className="flex items-center justify-between">
          <div className="space-y-0.5">
            <p className="text-sm font-bold">Thumbnail Generation</p>
            <p className="text-xs text-slate-500">
              Generate preview images for all processed links.
            </p>
          </div>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              type="checkbox"
              className="sr-only peer"
              checked={thumbnailGeneration}
              onChange={(e) => onThumbnailChange(e.target.checked)}
            />
            <div className="w-11 h-6 bg-slate-200 peer-focus:outline-none dark:bg-slate-700 peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-gray-300 after:border after:rounded-full after:h-5 after:w-5 after:transition-all peer-checked:bg-primary rounded-full" />
          </label>
        </div>
      </div>
    </section>
  );
}
