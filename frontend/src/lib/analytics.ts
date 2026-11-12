import log from "@/lib/log";

// posthog-js is ~59KB gzipped. It is loaded with a dynamic import once the
// browser is idle so it stays off every route's first-load bundle and out of
// the LCP critical path. Calls made before it loads are queued and replayed.
type PostHog = (typeof import("posthog-js"))["default"];
type PendingCall = (posthog: PostHog) => void;

const key = process.env.NEXT_PUBLIC_POSTHOG_KEY;
const host = process.env.NEXT_PUBLIC_POSTHOG_HOST;

// Upper bound on calls held while the SDK loads, so a load that never
// happens (e.g. blocked by an extension) can't grow memory without limit.
const MAX_QUEUE = 100;
// requestIdleCallback deadline: load even on a busy main thread eventually.
const IDLE_TIMEOUT_MS = 3000;
// Fallback delay where requestIdleCallback is unavailable (Safari).
const FALLBACK_DELAY_MS = 1000;

let client: PostHog | null = null;
let queue: PendingCall[] = [];
let initStarted = false;
let loadFailed = false;

function isEnabled(): boolean {
  return !!key;
}

function whenIdle(cb: () => void) {
  if (typeof window.requestIdleCallback === "function") {
    window.requestIdleCallback(cb, { timeout: IDLE_TIMEOUT_MS });
  } else {
    setTimeout(cb, FALLBACK_DELAY_MS);
  }
}

function run(call: PendingCall) {
  if (!isEnabled() || loadFailed) return;
  if (client) {
    call(client);
    return;
  }
  if (queue.length < MAX_QUEUE) queue.push(call);
}

async function load() {
  try {
    const { default: posthog } = await import("posthog-js");
    // TODO: consent — gate this behind a consent banner before wider rollout
    posthog.init(key!, {
      api_host: host,
      defaults: "2026-01-30",
      person_profiles: "identified_only",
      capture_exceptions: true,
      disable_surveys: true,
    });
    client = posthog;
    const pending = queue;
    queue = [];
    for (const call of pending) call(posthog);
  } catch (err) {
    loadFailed = true;
    queue = [];
    log.warn("analytics: failed to load posthog", { error: String(err) });
  }
}

const analytics = {
  init() {
    if (!isEnabled() || initStarted) return;
    initStarted = true;
    whenIdle(() => void load());
  },

  identify(userId: string, properties: Record<string, unknown>) {
    run((posthog) => posthog.identify(userId, properties));
  },

  reset() {
    run((posthog) => posthog.reset());
  },

  capture(event: string, properties?: Record<string, unknown>) {
    run((posthog) => posthog.capture(event, properties));
  },
};

export default analytics;
