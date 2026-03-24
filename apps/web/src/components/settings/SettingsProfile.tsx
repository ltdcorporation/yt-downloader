"use client";

import { useRef, type ChangeEvent } from "react";
import { Camera } from "@phosphor-icons/react";
import type { UserProfile } from "@/data/settings-data";

interface SettingsProfileProps {
  user: UserProfile;
  fullName: string;
  email: string;
  onFullNameChange: (value: string) => void;
  onAvatarUpload: (file: File) => Promise<void> | void;
  onAvatarRemove: () => Promise<void> | void;
  avatarBusy?: boolean;
  onEmailChange?: (value: string) => void;
  emailReadOnly?: boolean;
}

export default function SettingsProfile({
  user,
  fullName,
  email,
  onFullNameChange,
  onAvatarUpload,
  onAvatarRemove,
  avatarBusy = false,
  onEmailChange,
  emailReadOnly = false,
}: SettingsProfileProps) {
  const fileInputRef = useRef<HTMLInputElement | null>(null);

  const triggerFilePicker = () => {
    if (avatarBusy) {
      return;
    }
    fileInputRef.current?.click();
  };

  const handleFileChange = (event: ChangeEvent<HTMLInputElement>) => {
    const selectedFile = event.target.files?.[0];
    if (!selectedFile) {
      return;
    }

    void Promise.resolve(onAvatarUpload(selectedFile));
    event.target.value = "";
  };

  const handleRemove = () => {
    if (avatarBusy) {
      return;
    }
    void Promise.resolve(onAvatarRemove());
  };

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
              type="button"
              onClick={triggerFilePicker}
              disabled={avatarBusy}
              className="absolute bottom-0 right-0 bg-primary text-white p-1.5 rounded-full shadow-lg border-2 border-white dark:border-slate-900 hover:bg-primary/90 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
              aria-label="Change profile photo"
            >
              <Camera size={16} weight="fill" />
            </button>
            <input
              ref={fileInputRef}
              type="file"
              accept="image/png,image/jpeg,image/jpg,image/gif,image/webp"
              className="hidden"
              onChange={handleFileChange}
            />
          </div>
          <div className="space-y-1">
            <h4 className="font-bold">Profile Photo</h4>
            <p className="text-sm text-slate-500">JPG, PNG, GIF, or WEBP. Max size 2MB.</p>
            <div className="flex gap-2 mt-2">
              <button
                type="button"
                disabled={avatarBusy}
                onClick={triggerFilePicker}
                className="px-3 py-1.5 text-xs font-bold bg-primary text-white rounded-lg hover:bg-primary/90 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
              >
                {avatarBusy ? "Processing..." : "Upload New"}
              </button>
              <button
                type="button"
                disabled={avatarBusy}
                onClick={handleRemove}
                className="px-3 py-1.5 text-xs font-bold bg-slate-100 dark:bg-slate-800 text-slate-600 dark:text-slate-300 rounded-lg hover:bg-slate-200 dark:hover:bg-slate-700 transition-colors disabled:cursor-not-allowed disabled:opacity-60"
              >
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
              className="w-full px-4 py-2 bg-slate-50 dark:bg-slate-800 border-none rounded-lg focus:ring-2 focus:ring-primary/50 text-sm disabled:opacity-70 disabled:cursor-not-allowed"
              type="email"
              value={email}
              disabled={emailReadOnly}
              onChange={(e) => onEmailChange?.(e.target.value)}
            />
            {emailReadOnly ? (
              <p className="text-xs text-slate-500">
                Email updates require a verified flow and are currently read-only.
              </p>
            ) : null}
          </div>
        </div>
      </div>
    </section>
  );
}
