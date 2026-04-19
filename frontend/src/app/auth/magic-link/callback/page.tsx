"use client";

import { Suspense, useEffect } from "react";
import { useSearchParams } from "next/navigation";
import { getMagicLinkCallbackUrl } from "@/lib/api";

export default function MagicLinkCallbackPage() {
  return (
    <Suspense>
      <MagicLinkCallbackContent />
    </Suspense>
  );
}

function MagicLinkCallbackContent() {
  const searchParams = useSearchParams();
  const token = searchParams.get("token");

  useEffect(() => {
    if (token) {
      window.location.href = getMagicLinkCallbackUrl(token);
    }
  }, [token]);

  if (!token) {
    return <p>Invalid link.</p>;
  }

  return <p>Signing you in...</p>;
}
