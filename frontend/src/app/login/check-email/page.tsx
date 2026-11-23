"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { ApiError, sendMagicLink } from "@/lib/api";
import { safeReturnPath } from "@/lib/safeReturnPath";
import Button from "@/components/Button/Button";
import styles from "../page.module.css";
import checkStyles from "./page.module.css";

const RESEND_COOLDOWN_SECONDS = 30;

// The domain stays on one line; only the local part may wrap.
function EmailAddress({ email }: { email: string }) {
  const at = email.lastIndexOf("@");
  if (at < 0) return <>{email}</>;
  return (
    <>
      {email.slice(0, at)}
      <wbr />
      <span className={checkStyles.domain}>{email.slice(at)}</span>
    </>
  );
}

export default function CheckEmailPage() {
  return (
    <Suspense>
      <CheckEmailContent />
    </Suspense>
  );
}

function CheckEmailContent() {
  const searchParams = useSearchParams();
  const email = searchParams.get("email");
  const returnUrl = safeReturnPath(searchParams.get("return_url"));
  const loginHref =
    returnUrl === "/" ? "/login" : `/login?return_url=${encodeURIComponent(returnUrl)}`;

  // The link was sent just before landing here, so the cooldown starts now.
  const [cooldown, setCooldown] = useState(RESEND_COOLDOWN_SECONDS);
  const [resending, setResending] = useState(false);
  const [status, setStatus] = useState<{ kind: "sent" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (cooldown <= 0) return;
    const t = setTimeout(() => setCooldown((c) => c - 1), 1000);
    return () => clearTimeout(t);
  }, [cooldown]);

  async function handleResend() {
    if (!email) return;
    setStatus(null);
    setResending(true);
    try {
      await sendMagicLink(email, returnUrl);
      setStatus({ kind: "sent", text: "Link sent again." });
      setCooldown(RESEND_COOLDOWN_SECONDS);
    } catch (err) {
      setStatus({
        kind: "error",
        text: err instanceof ApiError ? err.message : "Something went wrong. Please try again.",
      });
    } finally {
      setResending(false);
    }
  }

  return (
    <div className={styles.page}>
      <main id="main" className={styles.container}>
        <h1 className={styles.heading}>Check your email</h1>
        <p className={styles.subheading}>
          We sent a sign-in link to{" "}
          <strong className={checkStyles.email}>
            {email ? <EmailAddress email={email} /> : "your email"}
          </strong>{" "}
          Click the link to sign in.
        </p>
        <p className={checkStyles.hint}>
          The link expires in 15 minutes. If you don&rsquo;t see it, check your spam folder.
        </p>
        {email ? (
          <Button
            variant="secondary"
            size="lg"
            fullWidth
            onClick={handleResend}
            loading={resending}
            disabled={cooldown > 0}
          >
            {cooldown > 0 ? `Resend link (${cooldown}s)` : "Resend link"}
          </Button>
        ) : null}
        {status ? (
          <p
            className={status.kind === "error" ? checkStyles.error : checkStyles.hint}
            role={status.kind === "error" ? "alert" : "status"}
          >
            {status.text}
          </p>
        ) : null}
        <a href={loginHref} className={checkStyles.different}>
          Use a different email
        </a>
      </main>
    </div>
  );
}
