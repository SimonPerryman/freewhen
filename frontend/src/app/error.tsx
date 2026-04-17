"use client";

import { useEffect } from "react";
import log from "@/lib/log";

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
    <div
      style={{
        display: "flex",
        minHeight: "100vh",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        fontFamily: "var(--font-geist-sans), sans-serif",
        backgroundColor: "var(--page-bg)",
        color: "var(--text-primary)",
      }}
    >
      <div
        style={{
          width: "100%",
          maxWidth: 480,
          textAlign: "center",
        }}
      >
        <h1
          style={{
            margin: "0 0 8px",
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: "-0.5px",
          }}
        >
          Something went wrong
        </h1>
        <p
          style={{
            margin: "0 0 28px",
            fontSize: 15,
            color: "var(--text-secondary)",
          }}
        >
          An unexpected error occurred. Please try again.
        </p>
        <button
          onClick={reset}
          style={{
            padding: "12px 16px",
            fontSize: 15,
            fontWeight: 500,
            fontFamily: "inherit",
            color: "var(--accent-text)",
            background: "var(--accent)",
            border: "none",
            borderRadius: 8,
            cursor: "pointer",
          }}
        >
          Try again
        </button>
      </div>
    </div>
  );
}
