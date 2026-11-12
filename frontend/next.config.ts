import type { NextConfig } from "next";

// Vercel defaults Production/Preview env vars to "sensitive", which makes them
// write-only — `vercel pull` writes "" to .env, the build inlines "", and the
// deployed bundle ships broken values. Fail loudly here instead.
//
// We only flag vars that are *defined-but-empty* (the sensitive trap). A truly
// undefined var means the project is intentionally not configuring it (e.g.
// PostHog disabled), which is a different choice and not an error.
//
// `NEXT_PUBLIC_API_URL` is required outright — it has no safe default.
//
// See docs/deploy-checklist.md.
if (process.env.VERCEL_ENV === "production" || process.env.VERCEL_ENV === "preview") {
  const REQUIRED = ["NEXT_PUBLIC_API_URL"] as const;
  const OPTIONAL_BUT_NOT_EMPTY = [
    "NEXT_PUBLIC_LOG_LEVEL",
    "NEXT_PUBLIC_POSTHOG_KEY",
    "NEXT_PUBLIC_POSTHOG_HOST",
  ] as const;

  const broken: string[] = [];
  for (const name of REQUIRED) {
    if (!process.env[name]) broken.push(name);
  }
  for (const name of OPTIONAL_BUT_NOT_EMPTY) {
    if (process.env[name] === "") broken.push(name);
  }

  if (broken.length > 0) {
    throw new Error(
      `Vercel build has empty NEXT_PUBLIC_* env var(s): ${broken.join(", ")}. ` +
        "Likely cause: the var was added without --no-sensitive, so `vercel pull` " +
        'cannot read it and writes "" to the build env. Run ' +
        "`vercel env rm <NAME> <env>` and re-add with `--no-sensitive`. " +
        "To intentionally disable an optional var, remove it entirely (do not set it to empty). " +
        "See docs/deploy-checklist.md.",
    );
  }
}

// --- Security headers -------------------------------------------------------
//
// Baseline headers are enforced. The full Content-Security-Policy ships in
// Report-Only mode for now: without per-request nonces, the App Router's
// inline hydration scripts and our inline colour-scheme script need
// 'unsafe-inline', and the PostHog / API origins are config-dependent, so an
// enforced policy risks breaking the app on a misconfigured deploy. Observe
// violations (browser console) in prod, then flip to enforcing — see
// plans/bugfix/frontend-open-redirects-and-headers.md.
//
// frame-ancestors is enforced separately (it has zero breakage risk and
// Report-Only would not actually stop clickjacking).

function originOf(url: string | undefined): string | null {
  if (!url) return null;
  try {
    return new URL(url).origin;
  } catch {
    return null;
  }
}

const apiOrigin = originOf(process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080");
const posthogOrigin = originOf(process.env.NEXT_PUBLIC_POSTHOG_HOST);
// PostHog serves its lazy-loaded extensions (recorder, surveys, toolbar) from
// *-assets.i.posthog.com regardless of the ingestion host.
const posthogWildcard = process.env.NEXT_PUBLIC_POSTHOG_KEY ? "https://*.posthog.com" : null;

function sources(...items: (string | null)[]): string {
  return [...new Set(items.filter((s): s is string => !!s))].join(" ");
}

const reportOnlyCsp = [
  "default-src 'self'",
  `script-src ${sources("'self'", "'unsafe-inline'", posthogOrigin, posthogWildcard)}`,
  "style-src 'self' 'unsafe-inline'",
  "img-src 'self' data: blob:",
  "font-src 'self' data:",
  // API fetches + realtime EventSource, PostHog ingestion.
  `connect-src ${sources("'self'", apiOrigin, posthogOrigin, posthogWildcard)}`,
  "worker-src 'self' blob:",
  "object-src 'none'",
  "base-uri 'self'",
  // Magic-link + login forms post via fetch; the Google sign-in flow is a
  // top-level navigation (not a form submit), so 'self' is enough here.
  "form-action 'self'",
  "frame-ancestors 'none'",
].join("; ");

const securityHeaders = [
  { key: "X-Frame-Options", value: "DENY" },
  { key: "Content-Security-Policy", value: "frame-ancestors 'none'" },
  { key: "Content-Security-Policy-Report-Only", value: reportOnlyCsp },
  // The magic-link callback page narrows this further to no-referrer via its
  // own metadata (token is in the URL).
  { key: "Referrer-Policy", value: "strict-origin-when-cross-origin" },
  { key: "X-Content-Type-Options", value: "nosniff" },
  // Deny powerful features we don't use. clipboard-write / web-share stay at
  // their default (self) — ShareButton needs them.
  {
    key: "Permissions-Policy",
    value: "camera=(), microphone=(), geolocation=(), payment=(), usb=()",
  },
];

const nextConfig: NextConfig = {
  async headers() {
    return [{ source: "/:path*", headers: securityHeaders }];
  },
};

export default nextConfig;
