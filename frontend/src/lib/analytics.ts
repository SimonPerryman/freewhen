import posthog from "posthog-js";

const key = process.env.NEXT_PUBLIC_POSTHOG_KEY;
const host = process.env.NEXT_PUBLIC_POSTHOG_HOST;

let initialised = false;

function isEnabled(): boolean {
  return !!key;
}

const analytics = {
  init() {
    if (!isEnabled() || initialised) return;
    initialised = true;

    // TODO: consent — gate this behind a consent banner before wider rollout
    posthog.init(key!, {
      api_host: host,
      defaults: "2026-01-30",
      person_profiles: "identified_only",
      capture_exceptions: true,
    });
  },

  identify(userId: string, properties: Record<string, unknown>) {
    if (!isEnabled()) return;
    posthog.identify(userId, properties);
  },

  reset() {
    if (!isEnabled()) return;
    posthog.reset();
  },

  capture(event: string, properties?: Record<string, unknown>) {
    if (!isEnabled()) return;
    posthog.capture(event, properties);
  },
};

export default analytics;
