import type { Metadata } from "next";

// Audit F16: the magic-link callback receives the token in `?token=...`. Even
// though the page currently loads no third-party assets, pinning the page-
// scoped Referrer-Policy to no-referrer means a future addition (analytics,
// fonts, speed-insights) can't silently leak the token via Referer before
// the user clicks Continue. The backend also sets Referrer-Policy globally;
// this is a belt-and-braces measure rendered into the document itself.
export const metadata: Metadata = {
  title: "Sign in — FreeWhen",
  referrer: "no-referrer",
};

export default function MagicLinkCallbackLayout({ children }: { children: React.ReactNode }) {
  return children;
}
