"use client";

import { Suspense, useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";
import { unsubscribe } from "@/lib/api";
import styles from "./page.module.css";

type Status = "pending" | "success" | "invalid";

function UnsubscribeInner() {
  const params = useSearchParams();
  const token = params.get("t");
  const [status, setStatus] = useState<Status>("pending");

  useEffect(() => {
    if (!token) {
      setStatus("invalid");
      return;
    }
    let cancelled = false;
    void unsubscribe(token)
      .then((ok) => {
        if (!cancelled) setStatus(ok ? "success" : "invalid");
      })
      .catch(() => {
        if (!cancelled) setStatus("invalid");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <main className={styles.main}>
      {status === "pending" ? (
        <p className={styles.body}>One moment…</p>
      ) : status === "success" ? (
        <>
          <h1 className={styles.heading}>You&apos;re unsubscribed</h1>
          <p className={styles.body}>
            We won&apos;t email you about this calendar again. You can edit your participant on
            FreeWhen if you change your mind.
          </p>
        </>
      ) : (
        <>
          <h1 className={styles.heading}>This unsubscribe link is invalid or expired.</h1>
          <p className={styles.body}>
            Open FreeWhen and edit your participant to clear your email.
          </p>
        </>
      )}
    </main>
  );
}

export default function UnsubscribePage() {
  return (
    <Suspense fallback={null}>
      <UnsubscribeInner />
    </Suspense>
  );
}
