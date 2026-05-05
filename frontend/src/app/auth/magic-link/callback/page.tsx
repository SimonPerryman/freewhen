"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ApiError, verifyMagicLink } from "@/lib/api";
import { log } from "@/lib/log";

export default function MagicLinkCallbackPage() {
  return (
    <Suspense>
      <MagicLinkCallbackContent />
    </Suspense>
  );
}

// Interstitial pattern (mitigates F4 in the auth audit): we deliberately do
// NOT consume the token on page load. Email link prefetchers — Outlook
// SafeLinks, Slack unfurl, corporate AV scanners — fetch the URL once on
// receipt and don't run JS or follow user-initiated POSTs, so the token
// survives until the user clicks "Sign in" and the POST runs.
function MagicLinkCallbackContent() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!token) {
    return (
      <main className="mx-auto max-w-sm px-4 py-12 text-center">
        <h1 className="text-lg font-semibold">Invalid link</h1>
        <p className="mt-2 text-sm text-neutral-600">
          This sign-in link is missing a token. Try requesting a new one.
        </p>
      </main>
    );
  }

  async function onContinue() {
    if (!token) return;
    setSubmitting(true);
    setError(null);
    try {
      const { returnUrl } = await verifyMagicLink(token);
      window.location.href = returnUrl || "/";
    } catch (err) {
      const msg =
        err instanceof ApiError ? err.message : "Could not sign you in. Try again.";
      log.warn("magic-link verify failed", { error: msg });
      setError(msg);
      setSubmitting(false);
    }
  }

  return (
    <main className="mx-auto max-w-sm px-4 py-12 text-center">
      <h1 className="text-lg font-semibold">Sign in to FreeWhen</h1>
      <p className="mt-2 text-sm text-neutral-600">
        Click below to finish signing in.
      </p>
      <button
        type="button"
        onClick={onContinue}
        disabled={submitting}
        className="mt-6 w-full rounded-md bg-neutral-900 px-4 py-2 text-sm font-medium text-white disabled:opacity-50"
      >
        {submitting ? "Signing you in…" : "Continue"}
      </button>
      {error ? <p className="mt-4 text-sm text-red-600">{error}</p> : null}
    </main>
  );
}
