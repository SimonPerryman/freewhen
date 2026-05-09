"use client";

import { useEffect, useState } from "react";
import { useSearchParams } from "next/navigation";

const API_BASE = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";

type Status = "pending" | "success" | "invalid";

export default function UnsubscribePage() {
  const params = useSearchParams();
  const token = params.get("t");
  const [status, setStatus] = useState<Status>("pending");

  useEffect(() => {
    if (!token) {
      setStatus("invalid");
      return;
    }
    let cancelled = false;
    void fetch(`${API_BASE}/unsubscribe?t=${encodeURIComponent(token)}`, { method: "GET" })
      .then((res) => {
        if (cancelled) return;
        setStatus(res.ok ? "success" : "invalid");
      })
      .catch(() => {
        if (!cancelled) setStatus("invalid");
      });
    return () => {
      cancelled = true;
    };
  }, [token]);

  return (
    <main
      style={{
        fontFamily: "system-ui, sans-serif",
        maxWidth: 480,
        margin: "64px auto",
        padding: "0 20px",
        color: "#111",
      }}
    >
      {status === "pending" ? (
        <p style={{ color: "#555" }}>One moment…</p>
      ) : status === "success" ? (
        <>
          <h1 style={{ fontSize: 22 }}>You&apos;re unsubscribed</h1>
          <p style={{ lineHeight: 1.5, color: "#555" }}>
            We won&apos;t email you about this calendar again. You can edit your participant on
            FreeWhen if you change your mind.
          </p>
        </>
      ) : (
        <>
          <h1 style={{ fontSize: 22 }}>This unsubscribe link is invalid or expired.</h1>
          <p style={{ lineHeight: 1.5, color: "#555" }}>
            Open FreeWhen and edit your participant to clear your email.
          </p>
        </>
      )}
    </main>
  );
}
