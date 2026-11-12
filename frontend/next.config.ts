import type { NextConfig } from "next";
import { PHASE_DEVELOPMENT_SERVER } from "next/constants";

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
// The full Content-Security-Policy is enforced. Origins come from config
// (NEXT_PUBLIC_API_URL, NEXT_PUBLIC_POSTHOG_*), never hard-coded domains.
//
// script-src keeps 'unsafe-inline': the App Router's inline bootstrap/flight
// scripts and our static colour-scheme script need it, and the alternative —
// per-request nonces — would force every route to render dynamically (no
// static landing page, no cached HTML). Every other directive is pinned to
// exact origins. See plans/tech-debt/enforce-csp.md.

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
// PostHog serves remote config and lazy-loaded extensions from
// *-assets.i.posthog.com regardless of the ingestion host.
const posthogWildcard = process.env.NEXT_PUBLIC_POSTHOG_KEY ? "https://*.posthog.com" : null;

function sources(...items: (string | null | false)[]): string {
  return [...new Set(items.filter((s): s is string => !!s))].join(" ");
}

// `devServer` is the `next dev` phase — a property of the tool being run, not
// of the deploy environment, so every deployed build gets the same policy.
// React's dev build needs eval() for its debugging features, and Speed
// Insights loads its debug script from Vercel's CDN under `next dev`.
function contentSecurityPolicy(devServer: boolean): string {
  return [
    "default-src 'self'",
    `script-src ${sources(
      "'self'",
      "'unsafe-inline'",
      devServer && "'unsafe-eval'",
      devServer && "https://va.vercel-scripts.com",
      posthogOrigin,
      posthogWildcard,
    )}`,
    "style-src 'self' 'unsafe-inline'",
    "img-src 'self' data: blob:",
    "font-src 'self' data:",
    // API fetches + realtime EventSource, PostHog ingestion.
    `connect-src ${sources("'self'", apiOrigin, posthogOrigin, posthogWildcard)}`,
    "worker-src 'self' blob:",
    "object-src 'none'",
    "base-uri 'self'",
    // Every form submits via fetch; Google sign-in and the .ics export are
    // top-level navigations to the API, not form posts.
    "form-action 'self'",
    "frame-ancestors 'none'",
  ].join("; ");
}

function securityHeaders(devServer: boolean) {
  return [
    { key: "X-Frame-Options", value: "DENY" },
    { key: "Content-Security-Policy", value: contentSecurityPolicy(devServer) },
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
}

export default function nextConfig(phase: string): NextConfig {
  const devServer = phase === PHASE_DEVELOPMENT_SERVER;
  return {
    async headers() {
      return [{ source: "/:path*", headers: securityHeaders(devServer) }];
    },
  };
}
