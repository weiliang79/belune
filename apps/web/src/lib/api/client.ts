const BASE_URL = "/api";

const SAFE_METHODS = new Set(["GET", "HEAD", "OPTIONS"]);

// Endpoints that must not trigger auto-refresh on 401: refreshing them would
// be circular (refresh) or pointless (login/setup). Anything else gets a
// single retry after a successful refresh.
const NO_REFRESH_PATHS = new Set([
  "/auth/login",
  // The second login step belongs here for a sharper reason than the others: a
  // 401 means the code was wrong, and retrying after a refresh re-submits that
  // same wrong code — burning two of the five attempts and two lockout strikes
  // for one typo.
  "/auth/login/verify",
  "/auth/refresh",
  "/auth/logout",
  "/auth/setup",
]);

export class ApiError extends Error {
  status: number;
  retryAfter?: number;

  constructor(message: string, status: number, retryAfter?: number) {
    super(message);
    this.status = status;
    this.retryAfter = retryAfter;
    this.name = "ApiError";
  }
}

function readCsrfToken(): string | null {
  const match = document.cookie.match(/(?:^|;\s*)csrf_token=([^;]+)/);
  return match ? decodeURIComponent(match[1]) : null;
}

// Coalesce concurrent refresh attempts: if a request is already in flight,
// piggy-back on it instead of stampeding /api/auth/refresh.
let refreshInFlight: Promise<boolean> | null = null;

async function attemptRefresh(): Promise<boolean> {
  if (refreshInFlight) return refreshInFlight;
  refreshInFlight = (async () => {
    try {
      const headers: Record<string, string> = {
        "Content-Type": "application/json",
      };
      const csrf = readCsrfToken();
      if (csrf) headers["X-CSRF-Token"] = csrf;
      const res = await fetch(`${BASE_URL}/auth/refresh`, {
        method: "POST",
        headers,
        credentials: "include",
      });
      return res.ok;
    } catch {
      return false;
    } finally {
      refreshInFlight = null;
    }
  })();
  return refreshInFlight;
}

async function doFetch(
  method: string,
  path: string,
  body?: unknown,
): Promise<Response> {
  const headers: Record<string, string> = {
    "Content-Type": "application/json",
  };

  if (!SAFE_METHODS.has(method)) {
    const csrf = readCsrfToken();
    if (csrf) headers["X-CSRF-Token"] = csrf;
  }

  const options: RequestInit = {
    method,
    headers,
    credentials: "include",
  };

  if (body) {
    options.body = JSON.stringify(body);
  }

  return fetch(`${BASE_URL}${path}`, options);
}

async function request<T>(
  method: string,
  path: string,
  body?: unknown,
): Promise<T> {
  let res = await doFetch(method, path, body);

  // 401 → try to refresh once, then retry. Refresh and other auth endpoints
  // are excluded so we don't recurse.
  if (res.status === 401 && !NO_REFRESH_PATHS.has(path)) {
    const refreshed = await attemptRefresh();
    if (refreshed) {
      res = await doFetch(method, path, body);
    }
  }

  if (!res.ok) {
    const text = await res.text();
    let message: string;
    let retryAfter: number | undefined;
    try {
      const parsed = JSON.parse(text);
      message = parsed.error ?? text;
      if (typeof parsed.retry_after === "number") {
        retryAfter = parsed.retry_after;
      }
    } catch {
      message = text;
    }
    if (retryAfter === undefined) {
      const header = res.headers.get("Retry-After");
      if (header) {
        const n = Number(header);
        if (!Number.isNaN(n)) retryAfter = n;
      }
    }
    throw new ApiError(message, res.status, retryAfter);
  }

  // A successful response need not carry a body: 204 obviously, but also 202
  // (the TLS recheck only enqueues a probe). res.json() on an empty body throws
  // "Unexpected end of JSON input", which surfaces as a bogus error toast on an
  // action that in fact succeeded. Decide by what arrived, not by the status.
  const text = await res.text();
  if (text === "") {
    return undefined as T;
  }

  return JSON.parse(text) as T;
}

export const api = {
  get: <T>(path: string) => request<T>("GET", path),
  post: <T>(path: string, body?: unknown) => request<T>("POST", path, body),
  put: <T>(path: string, body?: unknown) => request<T>("PUT", path, body),
  patch: <T>(path: string, body?: unknown) => request<T>("PATCH", path, body),
  delete: <T>(path: string) => request<T>("DELETE", path),
};
