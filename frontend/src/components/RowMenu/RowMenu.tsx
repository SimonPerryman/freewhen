"use client";

import { useCallback, useEffect, useId, useRef, useState } from "react";
import styles from "./RowMenu.module.css";

export interface RowMenuItem {
  id: string;
  label: string;
  onSelect: () => void;
  destructive?: boolean;
}

interface Props {
  items: RowMenuItem[];
  triggerLabel: string;
}

export default function RowMenu({ items, triggerLabel }: Props) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef<HTMLButtonElement | null>(null);
  const menuRef = useRef<HTMLDivElement | null>(null);
  const menuId = useId();

  const close = useCallback(() => {
    setOpen(false);
  }, []);

  useEffect(() => {
    if (!open) return;

    const handleKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        e.preventDefault();
        close();
        triggerRef.current?.focus();
      }
    };

    const handlePointer = (e: PointerEvent) => {
      const t = e.target as Node | null;
      if (!t) return;
      if (menuRef.current?.contains(t)) return;
      if (triggerRef.current?.contains(t)) return;
      close();
    };

    document.addEventListener("keydown", handleKey);
    document.addEventListener("pointerdown", handlePointer);
    return () => {
      document.removeEventListener("keydown", handleKey);
      document.removeEventListener("pointerdown", handlePointer);
    };
  }, [open, close]);

  useEffect(() => {
    if (!open) return;
    const first = menuRef.current?.querySelector<HTMLElement>('[role="menuitem"]');
    first?.focus();
  }, [open]);

  return (
    <>
      <button
        ref={triggerRef}
        type="button"
        className={styles.trigger}
        aria-label={triggerLabel}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-controls={open ? menuId : undefined}
        onClick={(e) => {
          e.preventDefault();
          e.stopPropagation();
          setOpen((v) => !v);
        }}
      >
        <span aria-hidden="true">⋯</span>
      </button>
      {open ? (
        <>
          <div className={styles.scrim} onClick={close} aria-hidden="true" />
          <div
            ref={menuRef}
            id={menuId}
            className={styles.menu}
            role="menu"
            aria-label={triggerLabel}
          >
            {items.map((item) => (
              <button
                key={item.id}
                type="button"
                role="menuitem"
                className={`${styles.item} ${item.destructive ? styles.destructive : ""}`}
                onClick={(e) => {
                  e.preventDefault();
                  e.stopPropagation();
                  close();
                  item.onSelect();
                }}
              >
                {item.label}
              </button>
            ))}
          </div>
        </>
      ) : null}
    </>
  );
}
