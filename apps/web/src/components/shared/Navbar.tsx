"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import Link from "next/link";
import { TrayArrowDown, List } from "@phosphor-icons/react";
import LoginModal from "./LoginModal";
import SignupModal from "./SignupModal";
import { api, APIError } from "@/lib/api";
import { clearAuthSessionSnapshot } from "@/lib/auth-session";
import { DEFAULT_AVATAR_URL } from "@/data/settings-data";
import { useAuthStore } from "@/store";

export default function Navbar() {
  const [isDrawerOpen, setIsDrawerOpen] = useState(false);
  const {
    currentUser,
    isAuthChecking,
    isLoginModalOpen,
    isSignupModalOpen,
    setCurrentUser,
    setIsAuthChecking,
    setLoginModalOpen,
    setSignupModalOpen,
    logout,
  } = useAuthStore();

  const avatarURL = useMemo(() => {
    return currentUser?.avatar_url || DEFAULT_AVATAR_URL;
  }, [currentUser?.avatar_url]);

  const refreshAuthState = useCallback(async () => {
    try {
      const me = await api.me();
      setCurrentUser(me.user);
    } catch (error) {
      if (error instanceof APIError && error.code === "invalid_session") {
        clearAuthSessionSnapshot();
      }
      setCurrentUser(null);
    } finally {
      setIsAuthChecking(false);
    }
  }, [setCurrentUser, setIsAuthChecking]);

  useEffect(() => {
    void refreshAuthState();

    const handleAuthChange = () => {
      void refreshAuthState();
    };

    window.addEventListener("quicksnap:auth-changed", handleAuthChange);
    return () => {
      window.removeEventListener("quicksnap:auth-changed", handleAuthChange);
    };
  }, [refreshAuthState]);

  const handleLogout = async () => {
    try {
      await api.logout();
    } catch {
      // noop
    }
    logout();
    setIsDrawerOpen(false);
  };

  return (
    <>
      <header className="flex items-center justify-between whitespace-nowrap border-b border-primary/10 px-4 py-4 sm:px-6 lg:px-20">
        <div className="flex items-center gap-2 sm:gap-3">
          <div className="text-primary flex items-center justify-center">
            <TrayArrowDown size={28} weight="fill" className="sm:size-8" />
          </div>
          <h2 className="text-primary text-lg sm:text-xl font-bold tracking-tight">
            QuickSnap
          </h2>
        </div>

        {/* Desktop Navigation */}
        <nav className="hidden md:flex flex-1 justify-center gap-10">
          <Link
            className="text-slate-600 dark:text-slate-400 hover:text-primary transition-colors text-sm font-medium"
            href="/#features"
          >
            Features
          </Link>
          <Link
            className="text-slate-600 dark:text-slate-400 hover:text-primary transition-colors text-sm font-medium"
            href="/history"
          >
            History
          </Link>
          <Link
            className="text-slate-600 dark:text-slate-400 hover:text-primary transition-colors text-sm font-medium"
            href="/settings"
          >
            Settings
          </Link>
        </nav>

        {/* Desktop Buttons */}
        <div className="hidden md:flex gap-3 items-center">
          {currentUser ? (
            <>
              <div className="flex items-center gap-2 rounded-lg border border-primary/15 px-3 py-2">
                <div className="h-7 w-7 rounded-full overflow-hidden border border-primary/20 bg-primary/5">
                  {/* eslint-disable-next-line @next/next/no-img-element */}
                  <img
                    src={avatarURL}
                    alt={`${currentUser.full_name} avatar`}
                    className="h-full w-full object-cover"
                  />
                </div>
                <span className="max-w-[180px] truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
                  {currentUser.full_name}
                </span>
              </div>
              <button
                onClick={handleLogout}
                className="flex min-w-[84px] cursor-pointer items-center justify-center rounded-lg h-10 px-5 bg-primary/10 text-primary text-sm font-bold hover:bg-primary/20 transition-all"
              >
                Logout
              </button>
            </>
          ) : (
            <>
              <button
                onClick={() => setLoginModalOpen(true)}
                className="flex min-w-[84px] cursor-pointer items-center justify-center rounded-lg h-10 px-5 bg-primary/10 text-primary text-sm font-bold hover:bg-primary/20 transition-all"
                disabled={isAuthChecking}
              >
                Login
              </button>
              <button
                onClick={() => setSignupModalOpen(true)}
                className="flex min-w-[84px] cursor-pointer items-center justify-center rounded-lg h-10 px-5 bg-primary text-white text-sm font-bold shadow-lg shadow-primary/20 hover:brightness-110 transition-all disabled:opacity-60"
                disabled={isAuthChecking}
              >
                Sign Up
              </button>
            </>
          )}
        </div>

        {/* Mobile Hamburger Button */}
        <button
          className="md:hidden flex items-center justify-center p-2 text-slate-600 dark:text-slate-400 hover:text-primary transition-colors"
          onClick={() => setIsDrawerOpen(true)}
          aria-label="Open menu"
        >
          <List size={28} weight="bold" />
        </button>
      </header>

      {/* Mobile Drawer Menu */}
      {isDrawerOpen && (
        <>
          {/* Backdrop */}
          <div
            className="fixed inset-0 z-[9998] bg-black/50 backdrop-blur-sm md:hidden transition-opacity duration-300"
            onClick={() => setIsDrawerOpen(false)}
          />

          {/* Drawer */}
          <div className="fixed top-0 right-0 z-[9999] h-full w-[280px] bg-white dark:bg-slate-900 shadow-2xl md:hidden transform transition-transform duration-300 ease-out animate-slide-in-right">
            <div className="flex flex-col h-full">
              {/* Drawer Header */}
              <div className="flex items-center justify-between px-6 py-4 border-b border-primary/10">
                <div className="flex items-center gap-2">
                  <TrayArrowDown size={24} weight="fill" className="text-primary" />
                  <span className="text-primary text-lg font-bold">QuickSnap</span>
                </div>
                <button
                  onClick={() => setIsDrawerOpen(false)}
                  className="p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
                  aria-label="Close menu"
                >
                  <List size={24} weight="bold" className="rotate-90" />
                </button>
              </div>

              {/* Drawer Navigation Links */}
              <nav className="flex-1 px-6 py-6 space-y-2 overflow-y-auto">
                <Link
                  href="/#features"
                  onClick={() => setIsDrawerOpen(false)}
                  className="block px-4 py-3 text-slate-600 dark:text-slate-400 hover:text-primary hover:bg-primary/5 rounded-lg transition-all text-base font-medium"
                >
                  Features
                </Link>
                <Link
                  href="/history"
                  onClick={() => setIsDrawerOpen(false)}
                  className="block px-4 py-3 text-slate-600 dark:text-slate-400 hover:text-primary hover:bg-primary/5 rounded-lg transition-all text-base font-medium"
                >
                  History
                </Link>
                <Link
                  href="/settings"
                  onClick={() => setIsDrawerOpen(false)}
                  className="block px-4 py-3 text-slate-600 dark:text-slate-400 hover:text-primary hover:bg-primary/5 rounded-lg transition-all text-base font-medium"
                >
                  Settings
                </Link>
              </nav>

              {/* Drawer Footer Buttons */}
              <div className="px-6 py-6 border-t border-primary/10 space-y-3">
                {currentUser ? (
                  <>
                    <div className="flex items-center gap-3 rounded-lg border border-primary/15 px-3 py-2">
                      <div className="h-8 w-8 rounded-full overflow-hidden border border-primary/20 bg-primary/5">
                        {/* eslint-disable-next-line @next/next/no-img-element */}
                        <img
                          src={avatarURL}
                          alt={`${currentUser.full_name} avatar`}
                          className="h-full w-full object-cover"
                        />
                      </div>
                      <div className="min-w-0">
                        <p className="truncate text-sm font-semibold text-slate-700 dark:text-slate-200">
                          {currentUser.full_name}
                        </p>
                        <p className="truncate text-xs text-slate-500 dark:text-slate-400">
                          {currentUser.email}
                        </p>
                      </div>
                    </div>
                    <button
                      onClick={handleLogout}
                      className="w-full flex items-center justify-center rounded-lg h-11 bg-primary/10 text-primary text-base font-bold hover:bg-primary/20 transition-all"
                    >
                      Logout
                    </button>
                  </>
                ) : (
                  <>
                <button
                  onClick={() => {
                    setIsDrawerOpen(false);
                    setLoginModalOpen(true);
                  }}
                  className="w-full flex items-center justify-center rounded-lg h-11 bg-primary/10 text-primary text-base font-bold hover:bg-primary/20 transition-all disabled:opacity-60"
                  disabled={isAuthChecking}
                >
                  Login
                </button>
                <button
                  onClick={() => {
                    setIsDrawerOpen(false);
                    setSignupModalOpen(true);
                  }}
                  className="w-full flex items-center justify-center rounded-lg h-11 bg-primary text-white text-base font-bold shadow-lg shadow-primary/20 hover:brightness-110 transition-all disabled:opacity-60"
                  disabled={isAuthChecking}
                >
                  Sign Up
                </button>
                  </>
                )}
              </div>
            </div>
          </div>
        </>
      )}

      {/* Login Modal */}
      <LoginModal
        isOpen={isLoginModalOpen}
        onClose={() => setLoginModalOpen(false)}
        onSwitchToSignup={() => {
          setLoginModalOpen(false);
          setSignupModalOpen(true);
        }}
      />

      {/* Signup Modal */}
      <SignupModal
        isOpen={isSignupModalOpen}
        onClose={() => setSignupModalOpen(false)}
        onSwitchToLogin={() => {
          setSignupModalOpen(false);
          setLoginModalOpen(true);
        }}
      />
    </>
  );
}
