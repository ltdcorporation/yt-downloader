import { create } from "zustand";
import { type AuthUser } from "@/lib/api";
import {
  clearAuthSessionSnapshot,
  readAuthSessionSnapshot,
  clearAdminAuth,
} from "@/lib/auth-session";

interface AuthState {
  currentUser: AuthUser | null;
  isAuthChecking: boolean;
  isLoginModalOpen: boolean;
  isSignupModalOpen: boolean;
  setCurrentUser: (user: AuthUser | null) => void;
  setIsAuthChecking: (isChecking: boolean) => void;
  setLoginModalOpen: (isOpen: boolean) => void;
  setSignupModalOpen: (isOpen: boolean) => void;
  logout: () => void;
}

export const useAuthStore = create<AuthState>((set) => ({
  currentUser: readAuthSessionSnapshot()?.user || null,
  isAuthChecking: true,
  isLoginModalOpen: false,
  isSignupModalOpen: false,
  setCurrentUser: (user) => set({ currentUser: user }),
  setIsAuthChecking: (isChecking) => set({ isAuthChecking: isChecking }),
  setLoginModalOpen: (isOpen) => set({ isLoginModalOpen: isOpen }),
  setSignupModalOpen: (isOpen) => set({ isSignupModalOpen: isOpen }),
  logout: () => {
    clearAuthSessionSnapshot();
    clearAdminAuth();
    set({ currentUser: null });
  },
}));
