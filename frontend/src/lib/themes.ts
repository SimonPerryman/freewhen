export interface Theme {
  slug: string;
  label: string;
  swatch: string;
}

// The curated theme set. Slug is the only value stored server-side; label is
// used for accessible names, swatch is the picker dot colour (light-mode
// accent). The authoritative accent palettes live in globals.css keyed by
// `[data-theme="<slug>"]`. The backend has its own allow-list in
// internal/service/themes.go — keep the slug order identical.
export const THEMES: readonly Theme[] = [
  { slug: "peach", label: "Peach", swatch: "#e06449" },
  { slug: "ocean", label: "Ocean", swatch: "#3b7a8e" },
  { slug: "forest", label: "Forest", swatch: "#4a6b4a" },
  { slug: "plum", label: "Plum", swatch: "#8e4a6b" },
  { slug: "lemon", label: "Lemon", swatch: "#d4a72c" },
  { slug: "graphite", label: "Graphite", swatch: "#5a5754" },
];

export const DEFAULT_THEME = "peach";

export function isValidTheme(slug: string): boolean {
  return THEMES.some((t) => t.slug === slug);
}

export function resolveTheme(slug: string | undefined | null): string {
  if (slug && isValidTheme(slug)) return slug;
  return DEFAULT_THEME;
}
