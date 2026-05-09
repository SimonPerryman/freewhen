import { useEffect, useRef, useState } from "react";

type Options = {
  typeMs?: number;
  holdMs?: number;
  deleteMs?: number;
  paused?: boolean;
};

type Phase = "typing" | "holding" | "deleting";

/**
 * Cycles `phrases` with a typewriter effect, pausing while `paused` is true,
 * the tab is hidden, or the user prefers reduced motion.
 */
export function useTypewriterPlaceholder(
  phrases: ReadonlyArray<string>,
  { typeMs = 60, holdMs = 1800, deleteMs = 35, paused = false }: Options = {},
): string {
  const [text, setText] = useState(() => phrases[0] ?? "");
  const indexRef = useRef(0);
  const phaseRef = useRef<Phase>("holding");
  const charRef = useRef(phrases[0]?.length ?? 0);

  useEffect(() => {
    if (phrases.length === 0) return;

    let timer: ReturnType<typeof setTimeout> | null = null;
    let cancelled = false;

    const reducedMotion =
      typeof window !== "undefined" && typeof window.matchMedia === "function"
        ? window.matchMedia("(prefers-reduced-motion: reduce)")
        : null;

    const isHidden = () =>
      typeof document !== "undefined" && document.visibilityState === "hidden";

    const shouldPause = () =>
      paused || isHidden() || (reducedMotion?.matches ?? false);

    const tick = () => {
      if (cancelled) return;
      if (shouldPause()) {
        timer = setTimeout(tick, 200);
        return;
      }

      const phrase = phrases[indexRef.current] ?? "";
      const phase = phaseRef.current;

      if (phase === "typing") {
        const next = charRef.current + 1;
        charRef.current = next;
        setText(phrase.slice(0, next));
        if (next >= phrase.length) {
          phaseRef.current = "holding";
          timer = setTimeout(tick, holdMs);
        } else {
          timer = setTimeout(tick, typeMs);
        }
        return;
      }

      if (phase === "holding") {
        phaseRef.current = "deleting";
        timer = setTimeout(tick, deleteMs);
        return;
      }

      // deleting
      const next = Math.max(0, charRef.current - 1);
      charRef.current = next;
      setText(phrase.slice(0, next));
      if (next === 0) {
        indexRef.current = (indexRef.current + 1) % phrases.length;
        phaseRef.current = "typing";
        timer = setTimeout(tick, typeMs);
      } else {
        timer = setTimeout(tick, deleteMs);
      }
    };

    const onVisibility = () => {
      if (!isHidden() && timer === null) {
        timer = setTimeout(tick, 0);
      }
    };

    timer = setTimeout(tick, holdMs);
    document.addEventListener("visibilitychange", onVisibility);
    reducedMotion?.addEventListener?.("change", onVisibility);

    return () => {
      cancelled = true;
      if (timer !== null) clearTimeout(timer);
      document.removeEventListener("visibilitychange", onVisibility);
      reducedMotion?.removeEventListener?.("change", onVisibility);
    };
  }, [phrases, typeMs, holdMs, deleteMs, paused]);

  return text;
}

export default useTypewriterPlaceholder;
