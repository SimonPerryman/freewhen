"use client";

import { useCallback, useState } from "react";
import { THEMES, resolveTheme } from "@/lib/themes";
import styles from "./ThemePicker.module.css";

interface Props {
  currentTheme: string | undefined | null;
  onChange: (slug: string) => Promise<void> | void;
}

export default function ThemePicker({ currentTheme, onChange }: Props) {
  const active = resolveTheme(currentTheme);
  const [pending, setPending] = useState<string | null>(null);

  const handlePick = useCallback(
    async (slug: string) => {
      if (slug === active || pending) return;
      setPending(slug);
      try {
        await onChange(slug);
      } finally {
        setPending(null);
      }
    },
    [active, onChange, pending],
  );

  return (
    <div className={styles.wrapper}>
      <p className={styles.label}>Theme</p>
      <div className={styles.row} role="radiogroup" aria-label="Calendar theme">
        {THEMES.map((theme) => {
          const isActive = theme.slug === active;
          return (
            <button
              key={theme.slug}
              type="button"
              role="radio"
              aria-checked={isActive}
              aria-label={theme.label}
              className={`${styles.swatch} ${isActive ? styles.active : ""}`}
              style={{ backgroundColor: theme.swatch }}
              onClick={() => handlePick(theme.slug)}
              disabled={pending !== null && pending !== theme.slug}
            >
              {isActive ? (
                <span className={styles.check} aria-hidden="true">
                  ✓
                </span>
              ) : null}
            </button>
          );
        })}
      </div>
    </div>
  );
}
