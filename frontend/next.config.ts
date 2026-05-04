import type { NextConfig } from "next";

// Vercel defaults Production/Preview env vars to "sensitive", which makes them
// write-only — `vercel pull` writes "" to .env, the build inlines "", and the
// deployed bundle calls same-origin paths instead of the API. Fail loudly here
// instead of shipping a broken client. See docs/deploy-checklist.md.
if (process.env.VERCEL_ENV === "production" || process.env.VERCEL_ENV === "preview") {
  if (!process.env.NEXT_PUBLIC_API_URL) {
    throw new Error(
      "NEXT_PUBLIC_API_URL is empty during a Vercel build. Likely cause: the var was added without --no-sensitive, so `vercel pull` cannot read it. Run `vercel env rm NEXT_PUBLIC_API_URL <env>` and re-add with `--no-sensitive`. See docs/deploy-checklist.md.",
    );
  }
}

const nextConfig: NextConfig = {
  /* config options here */
};

export default nextConfig;
