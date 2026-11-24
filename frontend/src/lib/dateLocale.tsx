"use client";

import { createContext, useContext, type ReactNode } from "react";

// The locale display dates are formatted in (see lib/dates.ts). Server-rendered
// pages provide the request's Accept-Language locale, which the browser derives
// from the same preference list as navigator.language, so the server's HTML and
// the hydrated client agree. Without a provider it's `undefined`: the runtime
// default, which in the browser is the browser's locale.
const DateLocaleContext = createContext<string | undefined>(undefined);

export function DateLocaleProvider({
  locale,
  children,
}: {
  locale: string | undefined;
  children: ReactNode;
}) {
  return <DateLocaleContext.Provider value={locale}>{children}</DateLocaleContext.Provider>;
}

export function useDateLocale(): string | undefined {
  return useContext(DateLocaleContext);
}
