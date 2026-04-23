import posthog from "posthog-js";

const key = process.env.NEXT_PUBLIC_POSTHOG_KEY;

function hasBackend(): boolean {
  return !!key;
}

export function isEnabled(flagKey: string, fallback = false): boolean {
  if (!hasBackend()) return fallback;
  const value = posthog.isFeatureEnabled(flagKey);
  return value ?? fallback;
}

export function getVariant(
  flagKey: string,
  fallback: string | null = null,
): string | null {
  if (!hasBackend()) return fallback;
  const value = posthog.getFeatureFlag(flagKey);
  if (typeof value === "string") return value;
  if (typeof value === "boolean") return value ? "true" : "false";
  return fallback;
}

export function onFlagsLoaded(cb: () => void): () => void {
  if (!hasBackend()) {
    cb();
    return () => {};
  }
  return posthog.onFeatureFlags(cb);
}
