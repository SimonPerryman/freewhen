"use client";

import { createContext, useContext, useSyncExternalStore, type ReactNode } from "react";
import { localToday } from "./localDate";

// "Today" (ISO `YYYY-MM-DD`) for components that render date-dependent
// markup — which week the grid starts on, which days count as past.
//
// Server-rendered pages hydrate with the *server's* today (passed down from
// the server component via <ServerTodayProvider>) so the hydration render
// matches the HTML exactly; React then re-renders with the browser's local
// date. Without this, a visitor whose timezone is on a different date than
// the server (UTC) would hit a hydration mismatch.
//
// Client-only renders just get the local date. getSnapshot runs on every
// render, so a long-lived tab still picks up a new day on its next render.

const ServerTodayContext = createContext<string | null>(null);

export function ServerTodayProvider({
  value,
  children,
}: {
  value: string | null | undefined;
  children: ReactNode;
}) {
  return (
    <ServerTodayContext.Provider value={value ?? null}>{children}</ServerTodayContext.Provider>
  );
}

function subscribe(): () => void {
  return () => {};
}

export function useToday(): string {
  const serverToday = useContext(ServerTodayContext);
  return useSyncExternalStore(subscribe, localToday, () => serverToday ?? localToday());
}
