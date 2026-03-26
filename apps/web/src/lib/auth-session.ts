import type { AuthResponse, AuthUser } from "./api";

const USER_KEY = "qs_auth_user";
const EXPIRES_AT_KEY = "qs_auth_expires_at";
const ADMIN_AUTH_KEY = "admin_auth";

export interface AuthSnapshot {
  user: AuthUser;
  expiresAt: string;
}

export function persistAdminAuth(user: AuthUser, credentialsBase64: string): void {
  if (typeof window === "undefined") {
    return;
  }
  window.sessionStorage.setItem(USER_KEY, JSON.stringify(user));
  window.sessionStorage.setItem(EXPIRES_AT_KEY, new Date(Date.now() + 24 * 60 * 60 * 1000).toISOString());
  window.sessionStorage.setItem(ADMIN_AUTH_KEY, credentialsBase64);
}

export function readAdminAuthCredentials(): string | null {
  if (typeof window === "undefined") {
    return null;
  }
  return window.sessionStorage.getItem(ADMIN_AUTH_KEY);
}

export function clearAdminAuth(): void {
  if (typeof window === "undefined") {
    return;
  }
  window.sessionStorage.removeItem(ADMIN_AUTH_KEY);
}

export function persistAuthSession(
  auth: AuthResponse,
  keepLoggedIn: boolean,
): void {
  if (typeof window === "undefined") {
    return;
  }

  const primaryStorage = keepLoggedIn
    ? window.localStorage
    : window.sessionStorage;
  const secondaryStorage = keepLoggedIn
    ? window.sessionStorage
    : window.localStorage;

  secondaryStorage.removeItem(USER_KEY);
  secondaryStorage.removeItem(EXPIRES_AT_KEY);

  primaryStorage.setItem(USER_KEY, JSON.stringify(auth.user));
  primaryStorage.setItem(EXPIRES_AT_KEY, auth.expires_at);
}

export function clearAuthSessionSnapshot(): void {
  if (typeof window === "undefined") {
    return;
  }

  window.localStorage.removeItem(USER_KEY);
  window.localStorage.removeItem(EXPIRES_AT_KEY);
  window.sessionStorage.removeItem(USER_KEY);
  window.sessionStorage.removeItem(EXPIRES_AT_KEY);
}

export function readAuthSessionSnapshot(): AuthSnapshot | null {
  if (typeof window === "undefined") {
    return null;
  }

  const storages = [window.sessionStorage, window.localStorage];
  for (const storage of storages) {
    const userRaw = storage.getItem(USER_KEY);
    const expiresAt = storage.getItem(EXPIRES_AT_KEY);
    if (!userRaw || !expiresAt) {
      continue;
    }

    try {
      const user = JSON.parse(userRaw) as AuthUser;
      return { user, expiresAt };
    } catch {
      storage.removeItem(USER_KEY);
      storage.removeItem(EXPIRES_AT_KEY);
    }
  }

  return null;
}
