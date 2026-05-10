"use client";

import { useCallback, useEffect, useState } from "react";
import {
  applyScheme,
  persistScheme,
  resolveInitialScheme,
  type ColorScheme,
} from "@/lib/colorScheme";
import styles from "./ColorSchemeToggle.module.css";

function SunIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <circle cx="8" cy="8" r="3" stroke="currentColor" strokeWidth="1.4" />
      <path
        d="M8 1.5V3M8 13V14.5M14.5 8H13M3 8H1.5M12.6 3.4L11.55 4.45M4.45 11.55L3.4 12.6M12.6 12.6L11.55 11.55M4.45 4.45L3.4 3.4"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinecap="round"
      />
    </svg>
  );
}

function MoonIcon() {
  return (
    <svg width="16" height="16" viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path
        d="M13.5 9.5A5.5 5.5 0 0 1 6.5 2.5a5.5 5.5 0 1 0 7 7Z"
        stroke="currentColor"
        strokeWidth="1.4"
        strokeLinejoin="round"
      />
    </svg>
  );
}

function useColorScheme(): [ColorScheme, (next: ColorScheme) => void] {
  const [scheme, setScheme] = useState<ColorScheme>("light");

  useEffect(() => {
    setScheme(resolveInitialScheme());
  }, []);

  const update = useCallback((next: ColorScheme) => {
    setScheme(next);
    applyScheme(next);
    persistScheme(next);
  }, []);

  return [scheme, update];
}

/** Segmented Light / Dark control — used inside the UserMenu panel. */
export function ColorSchemeSegmented() {
  const [scheme, setScheme] = useColorScheme();

  return (
    <div className={styles.segmented} role="radiogroup" aria-label="Appearance">
      <button
        type="button"
        role="radio"
        aria-checked={scheme === "light"}
        className={`${styles.segment} ${scheme === "light" ? styles.segmentActive : ""}`}
        onClick={() => setScheme("light")}
      >
        <SunIcon />
        <span>Light</span>
      </button>
      <button
        type="button"
        role="radio"
        aria-checked={scheme === "dark"}
        className={`${styles.segment} ${scheme === "dark" ? styles.segmentActive : ""}`}
        onClick={() => setScheme("dark")}
      >
        <MoonIcon />
        <span>Dark</span>
      </button>
    </div>
  );
}

/** Icon-only sun/moon toggle — used in Header for anonymous users. */
export function ColorSchemeIconButton() {
  const [scheme, setScheme] = useColorScheme();
  const next: ColorScheme = scheme === "dark" ? "light" : "dark";

  return (
    <button
      type="button"
      className={styles.iconButton}
      aria-label={`Switch to ${next} mode`}
      onClick={() => setScheme(next)}
    >
      {scheme === "dark" ? <SunIcon /> : <MoonIcon />}
    </button>
  );
}
