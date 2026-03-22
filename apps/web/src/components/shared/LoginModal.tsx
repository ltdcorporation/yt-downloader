"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  X,
  EnvelopeSimple,
  Lock,
  Eye,
  EyeSlash,
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

function resolveLoginErrorMessage(error: APIError): string {
  switch (error.code) {
    case "invalid_credentials":
      return "Email atau password salah.";
    case "google_auth_unavailable":
      return "Google login belum dikonfigurasi di server.";
    case "google_token_invalid":
      return "Token Google tidak valid. Coba login ulang.";
    case "google_email_unverified":
      return "Email Google kamu belum terverifikasi.";
    case "google_identity_conflict":
      return "Akun Google ini sudah terhubung ke user lain.";
    default:
      return error.message || "Login gagal. Coba lagi.";
  }
}

interface LoginModalProps {
  isOpen: boolean;
  onClose: () => void;
  onSwitchToSignup?: () => void;
}

export default function LoginModal({
  isOpen,
  onClose,
  onSwitchToSignup,
}: LoginModalProps) {
  const [email, setEmail] = useState("");
  const [password, setPassword] = useState("");
  const [showPassword, setShowPassword] = useState(false);
  const [keepLoggedIn, setKeepLoggedIn] = useState(false);
  const [isLoading, setIsLoading] = useState(false);
  const [isGoogleLoading, setIsGoogleLoading] = useState(false);
  const [errorMessage, setErrorMessage] = useState("");

  const googleButtonRef = useRef<HTMLDivElement>(null);
  const { setCurrentUser } = useAuthStore();

  const isGoogleConfigured = hasGoogleClientID();

  const resetForm = useCallback(() => {
    setEmail("");
    setPassword("");
    setShowPassword(false);
    setKeepLoggedIn(false);
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
          keepLoggedIn,
        });
        persistAuthSession(auth, keepLoggedIn);
        setCurrentUser(auth.user);
        window.dispatchEvent(new CustomEvent("quicksnap:auth-changed"));
        handleClose();
      } catch (error) {
        if (error instanceof APIError) {
          setErrorMessage(resolveLoginErrorMessage(error));
        } else {
          setErrorMessage(
            error instanceof Error ? error.message : "Login Google gagal.",
          );
        }
      } finally {
        setIsGoogleLoading(false);
      }
    };

    window.addEventListener("quicksnap:google-token", handleGoogleToken);

    const initGoogle = async () => {
      try {
        await warmupGoogleIdentity();
        if (googleButtonRef.current) {
          googleButtonRef.current.innerHTML = "";
          await renderGoogleButton(googleButtonRef.current, {
            text: "signin_with",
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
  }, [
    handleClose,
    isGoogleConfigured,
    isOpen,
    keepLoggedIn,
    setCurrentUser,
  ]);

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
      const auth = await api.login({
        email,
        password,
        keepLoggedIn,
      });
      persistAuthSession(auth, keepLoggedIn);
      setCurrentUser(auth.user);
      window.dispatchEvent(new CustomEvent("quicksnap:auth-changed"));
      handleClose();
    } catch (error) {
      if (error instanceof APIError) {
        setErrorMessage(resolveLoginErrorMessage(error));
      } else {
        setErrorMessage(
          error instanceof Error ? error.message : "Login gagal. Coba lagi.",
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
      <div className="bg-white dark:bg-slate-900 rounded-2xl w-full max-w-[440px] p-10 shadow-[0_10px_40px_-10px_rgba(0,0,0,0.08)] flex flex-col items-center relative animate-slide-up mx-4">
        {/* Close Button */}
        <button
          onClick={handleClose}
          className="absolute top-4 right-4 p-2 text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
          aria-label="Close modal"
        >
          <X size={24} weight="bold" />
        </button>

        {/* Top Icon */}
        <div className="w-12 h-12 bg-slate-100 dark:bg-slate-800 rounded-full flex items-center justify-center text-primary mb-6">
          <svg
            className="w-5 h-5"
            fill="none"
            stroke="currentColor"
            strokeWidth={1.5}
            viewBox="0 0 24 24"
            xmlns="http://www.w3.org/2000/svg"
          >
            <path
              d="M16.5 10.5V6.75a4.5 4.5 0 1 0-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 0 0 2.25-2.25v-6.75a2.25 2.25 0 0 0-2.25-2.25H6.75a2.25 2.25 0 0 0-2.25 2.25v6.75a2.25 2.25 0 0 0 2.25 2.25Z"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          </svg>
        </div>

        {/* Welcome Text */}
        <h1 className="text-2xl font-bold text-primary mb-2 text-center">
          Welcome Back
        </h1>
        <p className="text-slate-500 dark:text-slate-400 text-sm mb-8 text-center">
          Please enter your details to sign in
        </p>

        {/* Form */}
        <form className="w-full flex flex-col gap-5" onSubmit={handleSubmit}>
          {/* Email Field */}
          <div className="flex flex-col gap-1.5">
            <label
              className="text-sm font-semibold text-slate-900 dark:text-slate-100"
              htmlFor="email"
            >
              Email Address
            </label>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
                <EnvelopeSimple size={20} weight="fill" />
              </div>
              <input
                className="w-full pl-10 pr-4 py-3 border border-slate-200 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary focus:border-primary outline-none transition-all placeholder:text-slate-400 text-sm bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100"
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
            <div className="flex justify-between items-center">
              <label
                className="text-sm font-semibold text-slate-900 dark:text-slate-100"
                htmlFor="password"
              >
                Password
              </label>
              <a
                className="text-xs font-medium text-slate-500 dark:text-slate-400 hover:text-primary transition-colors"
                href="#"
              >
                Forgot Password?
              </a>
            </div>
            <div className="relative">
              <div className="absolute inset-y-0 left-0 pl-3 flex items-center pointer-events-none text-slate-400">
                <Lock size={20} weight="fill" />
              </div>
              <input
                className="w-full pl-10 pr-10 py-3 border border-slate-200 dark:border-slate-700 rounded-lg focus:ring-2 focus:ring-primary focus:border-primary outline-none transition-all placeholder:text-slate-400 text-sm font-bold tracking-widest bg-white dark:bg-slate-800 text-slate-900 dark:text-slate-100"
                id="password"
                placeholder="••••••••"
                type={showPassword ? "text" : "password"}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
                required
              />
              <button
                className="absolute inset-y-0 right-0 pr-3 flex items-center text-slate-400 hover:text-slate-600 dark:hover:text-slate-200 transition-colors"
                type="button"
                onClick={() => setShowPassword(!showPassword)}
                aria-label={showPassword ? "Hide password" : "Show password"}
              >
                {showPassword ? (
                  <EyeSlash size={20} weight="fill" />
                ) : (
                  <Eye size={20} weight="fill" />
                )}
              </button>
            </div>
          </div>

          {/* Keep me logged in */}
          <div className="flex items-center gap-2 mt-1">
            <input
              className="w-4 h-4 text-primary bg-white dark:bg-slate-800 border-slate-200 dark:border-slate-700 rounded focus:ring-primary focus:ring-2"
              id="keep-logged"
              type="checkbox"
              checked={keepLoggedIn}
              onChange={(e) => setKeepLoggedIn(e.target.checked)}
            />
            <label
              className="text-sm text-slate-500 dark:text-slate-400 cursor-pointer"
              htmlFor="keep-logged"
            >
              Keep me logged in
            </label>
          </div>

          {/* Error Message */}
          {errorMessage ? (
            <div className="rounded-lg border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700 dark:border-red-900/40 dark:bg-red-950/30 dark:text-red-300">
              {errorMessage}
            </div>
          ) : null}

          {/* Login Button */}
          <button
            className="w-full mt-2 py-3.5 bg-primary text-white font-semibold rounded-lg hover:brightness-110 transition-colors flex justify-center items-center gap-2 shadow-[0_4px_14px_0_rgba(128,126,152,0.39)] disabled:opacity-50 disabled:cursor-not-allowed"
            type="submit"
            disabled={isLoading || isGoogleLoading || !email || !password}
          >
            {isLoading ? (
              <span className="w-5 h-5 border-2 border-white border-t-transparent rounded-full animate-spin" />
            ) : (
              <>
                Login
                <ArrowRight size={16} weight="bold" />
              </>
            )}
          </button>
        </form>

        {/* Sign Up Link */}
        <div className="mt-6 text-sm text-slate-500 dark:text-slate-400 text-center">
          Don&apos;t have an account?{" "}
          <button
            className="font-semibold hover:text-primary transition-colors"
            type="button"
            onClick={() => {
              if (!onSwitchToSignup) {
                return;
              }
              resetForm();
              onSwitchToSignup();
            }}
          >
            Sign Up
          </button>
        </div>

        {/* Divider */}
        <div className="w-full relative flex items-center justify-center mt-6 mb-6">
          <div className="absolute w-full border-t border-slate-200 dark:border-slate-700" />
          <div className="bg-white dark:bg-slate-900 px-3 relative text-xs font-semibold text-slate-500 dark:text-slate-400 uppercase tracking-wider">
            Or continue with
          </div>
        </div>

        {/* Google Button */}
        <div ref={googleButtonRef} className="w-full min-h-[44px] flex justify-center" />

        {!isGoogleConfigured ? (
          <p className="mt-2 text-center text-xs text-slate-500 dark:text-slate-400">
            Google login belum aktif (NEXT_PUBLIC_GOOGLE_CLIENT_ID kosong).
          </p>
        ) : null}
      </div>
    </div>
  );
}
