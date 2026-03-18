"use client";

import { Camera } from "@phosphor-icons/react";
import type { UserProfile } from "@/data/settings-data";

interface SettingsProfileProps {
  user: UserProfile;
  fullName: string;
  email: string;
  onFullNameChange: (value: string) => void;
  onEmailChange: (value: string) => void;
}

export default function SettingsProfile({
  user,
  fullName,
  email,
  onFullNameChange,
  onEmailChange,
}: SettingsProfileProps) {
  return (
    <section className="bg-white dark:bg-slate-900 rounded-xl border border-slate-200 dark:border-slate-800 overflow-hidden">
      <div className="p-6 border-b border-slate-200 dark:border-slate-800">
        <h3 className="text-lg font-bold">Account Settings</h3>
        <p className="text-sm text-slate-500">
          Update your personal information and profile picture.
        </p>
      </div>
      <div className="p-6 space-y-6">
        <div className="flex items-center gap-6">
          <div className="relative group">
            <div className="size-24 rounded-full bg-slate-100 dark:bg-slate-800 overflow-hidden border-4 border-white dark:border-slate-700 shadow-sm">
              {/* eslint-disable-next-line @next/next/no-img-element */}
              <img
                className="w-full h-full object-cover"
                src={user.avatar}
                alt="Profile picture"
              />
            </div>
            <button
              className="absolute bottom-0 right-0 bg-primary text-white p-1.5 rounded-full shadow-lg border-2 border-white dark:border-slate-900 hover:bg-primary/90 transition-colors"
              aria-label="Change profile photo"
            >
              <Camera size={16} weight="fill" />
            </button>
          </div>
          <div className="space-y-1">
            <h4 className="font-bold">Profile Photo</h4>
            <p className="text-sm text-slate-500">
              JPG, GIF or PNG. Max size of 800K
            </p>
            <div className="flex gap-2 mt-2">
              <button className="px-3 py-1.5 text-xs font-bold bg-primary text-white rounded-lg hover:bg-primary/90 transition-colors">
                Upload New
              </button>
              <button className="px-3 py-1.5 text-xs font-bold bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors">
                Remove
              </button>
            </div>
          </div>
        </div>
        <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
          <div className="space-y-2">
            <label className="text-sm font-bold text-slate-700 dark:text-slate-300">
              Full Name
            </label>
            <input
              className="w-full px-4 py-2 bg-slate-50 dark:bg-slate-800 border-none rounded-lg focus:ring-2 focus:ring-primary/50 text-sm"
              type="text"
              value={fullName}
              onChange={(e) => onFullNameChange(e.target.value)}
            />
          </div>
          <div className="space-y-2">
            <label className="text-sm font-bold text-slate-700 dark:text-slate-300">
              Email Address
            </label>
            <input
              className="w-full px-4 py-2 bg-slate-50 dark:bg-slate-800 border-none rounded-lg focus:ring-2 focus:ring-primary/50 text-sm"
              type="email"
              value={email}
              onChange={(e) => onEmailChange(e.target.value)}
            />
          </div>
        </div>
      </div>
    </section>
  );
}
