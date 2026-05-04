import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "Sign in — FreeWhen",
  description: "Sign in to create and manage your FreeWhen calendars.",
};

export default function LoginLayout({ children }: { children: React.ReactNode }) {
  return children;
}
