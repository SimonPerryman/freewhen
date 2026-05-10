export type ColorScheme = "light" | "dark";

export const COLOR_SCHEME_STORAGE_KEY = "colorScheme";

export function readStoredScheme(): ColorScheme | null {
  if (typeof window === "undefined") return null;
  try {
    const raw = window.localStorage.getItem(COLOR_SCHEME_STORAGE_KEY);
    return raw === "light" || raw === "dark" ? raw : null;
  } catch {
    return null;
  }
}

export function applyScheme(scheme: ColorScheme): void {
  if (typeof document === "undefined") return;
  document.documentElement.dataset.colorScheme = scheme;
}

export function persistScheme(scheme: ColorScheme): void {
  if (typeof window === "undefined") return;
  try {
    window.localStorage.setItem(COLOR_SCHEME_STORAGE_KEY, scheme);
  } catch {
    /* ignore quota / privacy-mode failures */
  }
}

export function resolveInitialScheme(): ColorScheme {
  const stored = readStoredScheme();
  if (stored) return stored;
  if (
    typeof window !== "undefined" &&
    window.matchMedia?.("(prefers-color-scheme: dark)").matches
  ) {
    return "dark";
  }
  return "light";
}

/**
 * Inline blocking script string — runs in <head> before paint to set
 * data-color-scheme on <html>, preventing FOUC. Reads localStorage and
 * falls back to prefers-color-scheme.
 */
export const COLOR_SCHEME_INIT_SCRIPT = `(function(){try{var k='${COLOR_SCHEME_STORAGE_KEY}';var s=localStorage.getItem(k);if(s!=='light'&&s!=='dark'){s=window.matchMedia&&window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';}document.documentElement.dataset.colorScheme=s;}catch(e){}})();`;
