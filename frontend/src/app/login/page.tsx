"use client";

import { Suspense, useEffect, useState } from "react";
import { useRouter, useSearchParams } from "next/navigation";
import { getGoogleLoginUrl, sendMagicLink, ApiError } from "@/lib/api";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import { safeReturnPath } from "@/lib/safeReturnPath";
import styles from "./page.module.css";

export default function LoginPage() {
  return (
    <Suspense>
      <LoginContent />
    </Suspense>
  );
}

function LoginContent() {
  const router = useRouter();
  const searchParams = useSearchParams();
  const { user, checkAuth } = useAuth();
  // Never follow the raw query value — `/login?return_url=//evil.com` would
  // otherwise bounce an already-signed-in user off-site.
  const returnUrl = safeReturnPath(searchParams.get("return_url"));

  const [email, setEmail] = useState("");
  const [sending, setSending] = useState(false);
  const [error, setError] = useState("");

  useEffect(() => {
    checkAuth().then((u) => {
      if (u) router.replace(returnUrl);
    });
  }, [checkAuth, router, returnUrl]);

  if (user) return null;

  async function handleEmailSubmit(e: React.FormEvent) {
    e.preventDefault();
    setError("");
    setSending(true);

    try {
      await sendMagicLink(email, returnUrl);
      router.push(`/login/check-email?email=${encodeURIComponent(email)}`);
    } catch (err) {
      if (err instanceof ApiError) {
        setError(err.message);
      } else {
        setError("Something went wrong. Please try again.");
      }
      setSending(false);
    }
  }

  return (
    <div className={styles.page}>
      <main id="main" className={styles.container}>
        <h1 className={styles.heading}>Sign in</h1>
        <p className={styles.subheading}>Sign in to create and manage your calendars.</p>

        <a href={getGoogleLoginUrl(returnUrl)} className={styles.googleButton}>
          <svg className={styles.googleIcon} viewBox="0 0 24 24" aria-hidden="true">
            <path
              d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92a5.06 5.06 0 0 1-2.2 3.32v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.1z"
              fill="#4285F4"
            />
            <path
              d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
              fill="#34A853"
            />
            <path
              d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18A10.96 10.96 0 0 0 1 12c0 1.77.42 3.45 1.18 4.93l2.85-2.22.81-.62z"
              fill="#FBBC05"
            />
            <path
              d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
              fill="#EA4335"
            />
          </svg>
          Continue with Google
        </a>

        <div className={styles.divider}>
          <span className={styles.dividerText}>or</span>
        </div>

        <form onSubmit={handleEmailSubmit} className={styles.emailForm}>
          <label htmlFor="login-email" className={styles.emailLabel}>
            Email address
          </label>
          <input
            id="login-email"
            type="email"
            value={email}
            onChange={(e) => setEmail(e.target.value)}
            placeholder="you@example.com"
            className={styles.emailInput}
            required
            disabled={sending}
            aria-invalid={!!error}
            aria-describedby={error ? "login-email-error" : undefined}
          />
          <button type="submit" className={styles.emailButton} disabled={sending || !email}>
            {sending ? "Sending..." : "Send magic link"}
          </button>
          {error && (
            <p id="login-email-error" className={styles.error} role="alert">
              {error}
            </p>
          )}
        </form>
      </main>
    </div>
  );
}
