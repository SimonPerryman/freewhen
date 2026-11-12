"use client";

import { Suspense, useState } from "react";
import { useSearchParams } from "next/navigation";
import Button from "@/components/Button/Button";
import { ApiError, verifyMagicLink } from "@/lib/api";
import log from "@/lib/log";
import { safeReturnPath } from "@/lib/safeReturnPath";
import loginStyles from "@/app/login/page.module.css";
import styles from "./page.module.css";

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
// survives until the user clicks "Continue" and the POST runs.
function MagicLinkCallbackContent() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");
  const [submitting, setSubmitting] = useState(false);
  const [error, setError] = useState<string | null>(null);

  if (!token) {
    return (
      <div className={loginStyles.page}>
        <main className={loginStyles.container}>
          <h1 className={loginStyles.heading}>Invalid link</h1>
          <p className={styles.message}>
            This sign-in link is missing a token. Try requesting a new one.
          </p>
        </main>
      </div>
    );
  }

  async function onContinue() {
    if (!token) return;
    setSubmitting(true);
    setError(null);
    try {
      const { returnUrl } = await verifyMagicLink(token);
      // The backend already validates this, but re-check client-side so a
      // backend regression can't turn this into an open redirect.
      window.location.href = safeReturnPath(returnUrl);
    } catch (err) {
      const msg = err instanceof ApiError ? err.message : "Could not sign you in. Try again.";
      log.warn("magic-link verify failed", { error: msg });
      setError(msg);
      setSubmitting(false);
    }
  }

  return (
    <div className={loginStyles.page}>
      <main className={loginStyles.container}>
        <h1 className={loginStyles.heading}>Sign in to FreeWhen</h1>
        <p className={loginStyles.subheading}>Click below to finish signing in.</p>
        <Button variant="primary" size="lg" fullWidth loading={submitting} onClick={onContinue}>
          {submitting ? "Signing you in…" : "Continue"}
        </Button>
        {error ? (
          <p className={styles.error} role="alert">
            {error}
          </p>
        ) : null}
      </main>
    </div>
  );
}
