"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  X,
  User,
  EnvelopeSimple,
  Lock,
  ArrowRight,
} from "@phosphor-icons/react";
import { api, APIError } from "@/lib/api";
import { persistAuthSession } from "@/lib/auth-session";
import {
  hasGoogleClientID,
  warmupGoogleIdentity,
  renderGoogleButton,
} from "@/lib/google-identity";
import { useAuthStore } from "@/store";

function resolveSignupErrorMessage(error: APIError): string {
  switch (error.code) {
    case "email_taken":
      return "Email ini sudah terdaftar.";
    case "google_auth_unavailable":
      return "Google login belum dikonfigurasi di server.";
    case "google_token_invalid":
      return "Token Google tidak valid. Coba login ulang.";
    case "google_email_unverified":
      return "Email Google kamu belum terverifikasi.";
    case "google_identity_conflict":
      return "Akun Google ini sudah terhubung ke user lain.";
    default:
      return error.message || "Sign up gagal. Coba lagi.";
  }
}

interface SignupModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSwitchToLogin?: () => void;
}

export default function SignupModal({
  isOpen,
  onClose,
  onSwitchToLogin,
}: SignupModalProps) {
  const [fullName, setFullName] = useState("");
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isGoogleLoading, setIsGoogleLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const googleButtonRef = useRef<HTMLDivElement>(null);
  const { setCurrentUser } = useAuthStore();

  const isGoogleConfigured = hasGoogleClientID();

  const resetForm = useCallback(() => {
    setFullName("");
    setEmail("");
    setPassword("");
    setIsLoading(false);
    setIsGoogleLoading(false);
    setErrorMessage("");
  }, []);

  const handleClose = useCallback(() => {
    resetForm();
    onClose();
  }, [onClose, resetForm]);

  useEffect(() => {
    if (!isOpen || !isGoogleConfigured) {
      return;
    }

    const handleGoogleToken = async (e: any) => {
      const idToken = e.detail;
      if (!idToken) return;

      setIsGoogleLoading(true);
      setErrorMessage("");

      try {
        const auth = await api.loginWithGoogle({
          idToken,
          keepLoggedIn: true,
        });
        persistAuthSession(auth, true);
        setCurrentUser(auth.user);
        window.dispatchEvent(new CustomEvent("quicksnap:auth-changed"));
        handleClose();
      } catch (error) {
        if (error instanceof APIError) {
          setErrorMessage(resolveSignupErrorMessage(error));
        } else {
          setErrorMessage(
            error instanceof Error ? error.message : "Sign up Google gagal.",
          );
        }
      } finally {
        setIsGoogleLoading(false);
      }
    };

    window.addEventListener("quicksnap:google-token", handleGoogleToken);

    const initGoogle = async () => {
      // Small delay to ensure modal animation is far enough for DOM measurements
      await new Promise((resolve) => setTimeout(resolve, 100));
      try {
        await warmupGoogleIdentity();
        if (googleButtonRef.current) {
          googleButtonRef.current.innerHTML = "";
          await renderGoogleButton(googleButtonRef.current, {
            text: "signup_with",
          });
        }
      } catch (err) {
        console.error("Google init failed:", err);
      }
    };

    void initGoogle();

    return () => {
      window.removeEventListener("quicksnap:google-token", handleGoogleToken);
    };
  }, [handleClose, isGoogleConfigured, isOpen, setCurrentUser]);

  if (!isOpen) {
    return null;
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();

    if (isLoading || isGoogleLoading) {
      return;
    }

    setErrorMessage("");
    setIsLoading(true);

    try {
      const auth = await api.register({
        fullName,
        email,
        password,
        keepLoggedIn: true,
      });
      persistAuthSession(auth, true);
      setCurrentUser(auth.user);
      window.dispatchEvent(new CustomEvent("quicksnap:auth-changed"));
      handleClose();
    } catch (error) {
      if (error instanceof APIError) {
        setErrorMessage(resolveSignupErrorMessage(error));
      } else {
        setErrorMessage(
          error instanceof Error ? error.message : "Sign up gagal. Coba lagi.",
        );
      }
    } finally {
      setIsLoading(false);
    }
  };

  const handleBackdropClick = (e: React.MouseEvent<HTMLDivElement>) => {
    if (e.target === e.currentTarget) {
      handleClose();
    }
  };

  return (
    <div
      className="fixed inset-0 z-[9999] flex items-center justify-center bg-black/50 backdrop-blur-sm animate-fade-in"
      onClick={handleBackdropClick}
    >
      {/* Modal Content */}
      <div className="w-full max-w-[480px] bg-white dark:bg-slate-900 rounded-xl shadow-xl overflow-hidden border border-slate-200 dark:border-slate-800 relative animate-slide-up mx-4">
        {/* Header */}
        <div className="px-8 pt-8 pb-4 flex items-center justify-between">
          <div className="flex items-center gap-2">
            <div className="w-8 h-8 bg-primary rounded-lg flex items-center justify-center text-white">
              <svg
                className="w-5 h-5"
                fill="currentColor"
                viewBox="0 0 24 24"
                xmlns="http://www.w3.org/2000/svg"
              >
                <path d="M4.5 4.5a3 3 0 0 0-3 3v9a3 3 0 0 0 3 3h8.25a3 3 0 0 0 3-3v-9a3 3 0 0 0-3-3H4.5ZM19.94 18.75l-2.69-2.69V7.94l2.69-2.69c.944-.945 2.56-.276 2.56 1.06v11.38c0 1.336-1.616 2.005-2.56 1.06Z" />
              </svg>
            </div>
            <span className="text-slate-900 dark:text-slate-100 text-xl font-bold tracking-tight">
              QuickSnap
            </span>
          </div>
          <button
            onClick={handleClose}
            className="text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
            aria-label="Close modal"
          >
            <X size={24} weight="bold" />
          </button>
        </div>

        {/* Title Section */}
        <div className="px-8 pt-2 pb-6">
          <h1 className="text-2xl font-bold text-slate-900 dark:text-slate-100">
            Create Account
          </h1>
          <p className="text-slate-500 dark:text-slate-400 text-sm mt-1">
            Join QuickSnap to start downloading videos instantly.
          </p>
        </div>

        {/* Form */}
        <form className="px-8 pb-8 space-y-5" onSubmit={handleSubmit}>
          <div className="space-y-4">
            {/* Full Name Field */}
            <div className="flex flex-col gap-1.5">
              <label
                className="text-sm font-semibold text-slate-700 dark:text-slate-300"
                htmlFor="fullName"
              >
                Full Name
              </label>
              <div className="relative flex items-center">
                <User
                  size={20}
                  weight="fill"
                  className="absolute left-3 text-slate-400"
                />
                <input
                  className="w-full pl-10 pr-4 py-3 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all text-slate-900 dark:text-slate-100 placeholder:text-slate-400"
                  id="fullName"
                  placeholder="Enter your full name"
                  type="text"
                  value={fullName}
                  onChange={(e) => setFullName(e.target.value)}
                  required
                />
              </div>
            </div>

            {/* Email Field */}
            <div className="flex flex-col gap-1.5">
              <label
                className="text-sm font-semibold text-slate-700 dark:text-slate-300"
                htmlFor="email"
              >
                Email Address
              </label>
              <div className="relative flex items-center">
                <EnvelopeSimple
                  size={20}
                  weight="fill"
                  className="absolute left-3 text-slate-400"
                />
                <input
                  className="w-full pl-10 pr-4 py-3 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all text-slate-900 dark:text-slate-100 placeholder:text-slate-400"
                  id="email"
                  placeholder="name@example.com"
                  type="email"
                  value={email}
                  onChange={(e) => setEmail(e.target.value)}
                  required
                />
              </div>
            </div>

            {/* Password Field */}
            <div className="flex flex-col gap-1.5">
              <label
                className="text-sm font-semibold text-slate-700 dark:text-slate-300"
                htmlFor="password"
              >
                Password
              </label>
              <div className="relative flex items-center">
                <Lock
                  size={20}
                  weight="fill"
                  className="absolute left-3 text-slate-400"
                />
                <input
                  className="w-full pl-10 pr-4 py-3 bg-slate-50 dark:bg-slate-800 border border-slate-200 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary/20 focus:border-primary outline-none transition-all text-slate-900 dark:text-slate-100 placeholder:text-slate-400"
                  id="password"
                  placeholder="Create a password"
                  type="password"
                  value={password}
                  onChange={(e) => setPassword(e.target.value)}
                  required
                />
              </div>
            </div>
          </div>

          {/* Error Message */}
          {errorMessage ? (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300">
              {errorMessage}
            </div>
          ) : null}

          {/* Sign Up Button */}
          <button
            className="w-full py-3.5 bg-primary hover:bg-primary/90 text-white font-bold rounded-lg transition-colors shadow-lg shadow-primary/20 disabled:opacity-50 disabled:cursor-not-allowed"
            type="submit"
            disabled={isLoading || isGoogleLoading || !fullName || !email || !password}
          >
            {isLoading ? (
              <span className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
            ) : (
              <div className="flex items-center justify-center gap-2">
                Sign Up
                <ArrowRight size={16} weight="bold" />
              </div>
            )}
          </button>

          {/* Divider */}
          <div className="relative flex items-center py-2">
            <div className="flex-grow border-t border-slate-200 dark:border-slate-700" />
            <span className="flex-shrink mx-4 text-slate-400 text-xs font-medium uppercase tracking-wider">
              or
            </span>
            <div className="flex-grow border-t border-slate-200 dark:border-slate-700" />
          </div>

          {/* Google Button */}
          <div ref={googleButtonRef} className="w-full min-h-[44px] flex justify-center" />

          {!isGoogleConfigured ? (
            <p className="-mt-2 text-center text-xs text-slate-500 dark:text-slate-400">
              Google login belum aktif (NEXT_PUBLIC_GOOGLE_CLIENT_ID kosong).
            </p>
          ) : null}

          {/* Login Link */}
          <p className="text-center text-slate-500 dark:text-slate-400 text-sm">
            Already have an account?{" "}
            <button
              className="text-primary font-semibold hover:underline decoration-primary/30"
              type="button"
              onClick={() => {
                if (!onSwitchToLogin) {
                  return;
                }
                resetForm();
                onSwitchToLogin();
              }}
            >
              Login
            </button>
          </p>
        </form>
      </div>
    </div>
  );
}
