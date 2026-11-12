"use client";

import { useEffect, useRef, type RefObject } from "react";

const FOCUSABLE_SELECTOR =
  'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])';

interface Options {
  // Set to false to disable the trap (e.g. while submitting). Defaults to true.
  enabled?: boolean;
  // Called when the user presses Escape inside the trapped region.
  onEscape?: () => void;
  // Element to focus when the trap activates. Defaults to the first focusable
  // descendant of the container.
  initialFocus?: RefObject<HTMLElement | null>;
}

// Active traps, oldest first. Only the last (topmost) one handles keys, so
// Escape on a confirm dialog stacked over a sheet closes just the dialog.
const stack: object[] = [];

// Traps Tab/Shift-Tab focus inside the returned ref's container, handles
// Escape, and restores focus to whatever was active before the trap mounted.
// Used by every modal sheet/dialog so behaviour stays consistent.
export function useFocusTrap<T extends HTMLElement = HTMLDivElement>({
  enabled = true,
  onEscape,
  initialFocus,
}: Options = {}): RefObject<T | null> {
  const ref = useRef<T | null>(null);
  // Latest-ref pattern: callers typically pass a fresh arrow each render. If
  // onEscape were an effect dependency, every re-render would tear the trap
  // down (restoring focus) and set it up again (re-focusing initialFocus).
  const onEscapeRef = useRef(onEscape);
  useEffect(() => {
    onEscapeRef.current = onEscape;
  }, [onEscape]);

  useEffect(() => {
    if (!enabled) return;
    const root = ref.current;
    if (!root) return;
    const token = {};
    stack.push(token);
    const previouslyFocused = document.activeElement as HTMLElement | null;

    const target =
      initialFocus?.current ?? root.querySelector<HTMLElement>(FOCUSABLE_SELECTOR) ?? root;
    target.focus?.();

    function onKey(e: KeyboardEvent) {
      if (stack[stack.length - 1] !== token) return;
      if (e.key === "Escape") {
        const handler = onEscapeRef.current;
        if (handler) {
          e.stopPropagation();
          handler();
        }
        return;
      }
      if (e.key !== "Tab" || !root) return;
      const focusable = Array.from(root.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
        (el) => !el.hasAttribute("aria-hidden"),
      );
      if (focusable.length === 0) {
        e.preventDefault();
        return;
      }
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      const active = document.activeElement as HTMLElement | null;
      if (!root.contains(active)) {
        // Focus escaped (e.g. a click on the backdrop) — pull it back in.
        e.preventDefault();
        (e.shiftKey ? last : first).focus();
      } else if (e.shiftKey && active === first) {
        e.preventDefault();
        last.focus();
      } else if (!e.shiftKey && active === last) {
        e.preventDefault();
        first.focus();
      }
    }

    document.addEventListener("keydown", onKey);
    return () => {
      document.removeEventListener("keydown", onKey);
      const index = stack.indexOf(token);
      const wasTop = index === stack.length - 1;
      if (index !== -1) stack.splice(index, 1);
      // Restore only when this was the topmost trap — a lower layer closing
      // underneath an open dialog mustn't pull focus out of the dialog — and
      // only to an element still in the page (else focus would land on body).
      if (wasTop && previouslyFocused?.isConnected) previouslyFocused.focus?.();
    };
  }, [enabled, initialFocus]);

  return ref;
}
