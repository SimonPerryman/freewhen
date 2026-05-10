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

const nextConfig: NextConfig = {
  /* config options here */
};

export default nextConfig;
