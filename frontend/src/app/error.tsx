"use client";

import { useEffect } from "react";
import log from "@/lib/log";
import styles from "./status-page.module.css";

export default function Error({
  error,
  reset,
}: {
  error: Error & { digest?: string };
  reset: () => void;
}) {
  useEffect(() => {
    log.error("Unhandled error:", error);
  }, [error]);

  return (
    <div className={styles.page}>
      <div className={styles.inner}>
        <h1 className={styles.title}>Something went wrong</h1>
        <p className={styles.message}>An unexpected error occurred. Please try again.</p>
        <button type="button" onClick={reset} className={styles.action}>
          Try again
        </button>
      </div>
    </div>
  );
}
