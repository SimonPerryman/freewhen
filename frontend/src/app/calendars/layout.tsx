import type { Metadata } from "next";

export const metadata: Metadata = {
  title: "My calendars — FreeWhen",
  description: "Calendars you've created or joined on FreeWhen.",
};

export default function CalendarsLayout({ children }: { children: React.ReactNode }) {
  return children;
}
