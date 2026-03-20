const GOOGLE_IDENTITY_SCRIPT_ID = "google-identity-service-script";
const GOOGLE_IDENTITY_SCRIPT_SRC = "https://accounts.google.com/gsi/client";
const DEFAULT_TOKEN_TIMEOUT_MS = 30000;

const GOOGLE_CLIENT_ID =
  process.env.NEXT_PUBLIC_GOOGLE_CLIENT_ID?.trim() || "";

interface GoogleCredentialResponse {
  credential?: string;
}

interface GooglePromptMomentNotification {
  isNotDisplayed?: () => boolean;
  getNotDisplayedReason?: () => string;
  isSkippedMoment?: () => boolean;
  getSkippedReason?: () => string;
  isDismissedMoment?: () => boolean;
  getDismissedReason?: () => string;
}

interface GoogleInitializeOptions {
  client_id: string;
  callback: (response: GoogleCredentialResponse) => void;
  auto_select?: boolean;
  cancel_on_tap_outside?: boolean;
  itp_support?: boolean;
  use_fedcm_for_prompt?: boolean;
}

interface GoogleAccountsIDAPI {
  initialize: (options: GoogleInitializeOptions) => void;
  prompt: (
    momentListener?: (notification: GooglePromptMomentNotification) => void,
  ) => void;
  cancel?: () => void;
}

interface GoogleNamespace {
  accounts: {
    id: GoogleAccountsIDAPI;
  };
}

declare global {
  interface Window {
    google?: GoogleNamespace;
  }
}

type PendingGoogleTokenRequest = {
  resolve: (token: string) => void;
  reject: (error: Error) => void;
  timeoutId: ReturnType<typeof setTimeout>;
};

let scriptLoadPromise: Promise<void> | null = null;
let initializedClientID = "";
let pendingTokenRequest: PendingGoogleTokenRequest | null = null;

function googleIdentityAvailable(): boolean {
  return Boolean(window.google?.accounts?.id);
}

function normalizeErrorMessage(error: unknown, fallback: string): Error {
  if (error instanceof Error) {
    return error;
  }
  if (typeof error === "string" && error.trim() !== "") {
    return new Error(error.trim());
  }
  return new Error(fallback);
}

function consumePendingRequestWithToken(token: string): void {
  if (!pendingTokenRequest) {
    return;
  }

  const current = pendingTokenRequest;
  pendingTokenRequest = null;
  clearTimeout(current.timeoutId);
  current.resolve(token);
}

function consumePendingRequestWithError(error: Error): void {
  if (!pendingTokenRequest) {
    return;
  }

  const current = pendingTokenRequest;
  pendingTokenRequest = null;
  clearTimeout(current.timeoutId);
  current.reject(error);
}

function onGoogleCredential(response: GoogleCredentialResponse): void {
  const token = response?.credential?.trim();
  if (!token) {
    consumePendingRequestWithError(
      new Error("Google tidak mengembalikan credential yang valid."),
    );
    return;
  }

  consumePendingRequestWithToken(token);
}

function buildPromptErrorMessage(
  notification: GooglePromptMomentNotification,
): string {
  if (notification.isNotDisplayed?.()) {
    const reason = notification.getNotDisplayedReason?.() || "unknown";
    return `Prompt Google tidak tampil (reason: ${reason}).`;
  }
  if (notification.isSkippedMoment?.()) {
    const reason = notification.getSkippedReason?.() || "unknown";
    return `Prompt Google dilewati (reason: ${reason}).`;
  }
  if (notification.isDismissedMoment?.()) {
    const reason = notification.getDismissedReason?.() || "unknown";
    return `Prompt Google ditutup (reason: ${reason}).`;
  }
  return "Prompt Google tidak tersedia di browser ini.";
}

async function loadGoogleIdentityScript(): Promise<void> {
  if (typeof window === "undefined") {
    throw new Error("Google login hanya bisa dipakai di browser.");
  }

  if (googleIdentityAvailable()) {
    return;
  }

  if (scriptLoadPromise) {
    return scriptLoadPromise;
  }

  scriptLoadPromise = new Promise<void>((resolve, reject) => {
    const existing = document.getElementById(
      GOOGLE_IDENTITY_SCRIPT_ID,
    ) as HTMLScriptElement | null;

    const script = existing || document.createElement("script");
    if (!existing) {
      script.id = GOOGLE_IDENTITY_SCRIPT_ID;
      script.src = GOOGLE_IDENTITY_SCRIPT_SRC;
      script.async = true;
      script.defer = true;
      document.head.appendChild(script);
    }

    const cleanup = () => {
      script.removeEventListener("load", onLoad);
      script.removeEventListener("error", onError);
      clearTimeout(timeoutId);
    };

    const onLoad = () => {
      cleanup();
      if (!googleIdentityAvailable()) {
        scriptLoadPromise = null;
        reject(new Error("Google Identity script loaded but API unavailable."));
        return;
      }
      resolve();
    };

    const onError = () => {
      cleanup();
      scriptLoadPromise = null;
      reject(new Error("Gagal memuat Google Identity script."));
    };

    const timeoutId = setTimeout(() => {
      cleanup();
      scriptLoadPromise = null;
      reject(new Error("Timeout saat memuat Google Identity script."));
    }, 15000);

    script.addEventListener("load", onLoad);
    script.addEventListener("error", onError);

    if (googleIdentityAvailable()) {
      onLoad();
    }
  });

  return scriptLoadPromise;
}

function ensureGoogleIdentityInitialized(clientID: string): void {
  if (!googleIdentityAvailable()) {
    throw new Error("Google Identity API belum siap.");
  }

  if (initializedClientID === clientID) {
    return;
  }

  window.google?.accounts.id.initialize({
    client_id: clientID,
    callback: onGoogleCredential,
    auto_select: false,
    cancel_on_tap_outside: true,
    itp_support: true,
    use_fedcm_for_prompt: true,
  });

  initializedClientID = clientID;
}

async function ensureReady(): Promise<void> {
  if (!hasGoogleClientID()) {
    throw new Error("Google login belum dikonfigurasi (NEXT_PUBLIC_GOOGLE_CLIENT_ID kosong).");
  }

  await loadGoogleIdentityScript();
  ensureGoogleIdentityInitialized(GOOGLE_CLIENT_ID);
}

export function hasGoogleClientID(): boolean {
  return GOOGLE_CLIENT_ID !== "";
}

export async function warmupGoogleIdentity(): Promise<void> {
  if (!hasGoogleClientID()) {
    return;
  }
  await ensureReady();
}

export async function requestGoogleIDToken(
  timeoutMs = DEFAULT_TOKEN_TIMEOUT_MS,
): Promise<string> {
  await ensureReady();

  if (pendingTokenRequest) {
    throw new Error("Proses login Google lain masih berjalan. Coba lagi sebentar.");
  }

  return new Promise<string>((resolve, reject) => {
    const timeoutId = setTimeout(() => {
      consumePendingRequestWithError(
        new Error("Login Google timeout. Coba lagi."),
      );
    }, timeoutMs);

    pendingTokenRequest = {
      resolve,
      reject,
      timeoutId,
    };

    try {
      window.google?.accounts.id.cancel?.();
      window.google?.accounts.id.prompt((notification) => {
        if (!pendingTokenRequest) {
          return;
        }

        if (
          notification.isNotDisplayed?.() ||
          notification.isSkippedMoment?.() ||
          notification.isDismissedMoment?.()
        ) {
          setTimeout(() => {
            if (!pendingTokenRequest) {
              return;
            }
            consumePendingRequestWithError(
              new Error(buildPromptErrorMessage(notification)),
            );
          }, 300);
        }
      });
    } catch (error) {
      consumePendingRequestWithError(
        normalizeErrorMessage(error, "Gagal memulai login Google."),
      );
    }
  });
}
