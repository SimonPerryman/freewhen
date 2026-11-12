import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";

const posthogMock = {
  init: vi.fn(),
  identify: vi.fn(),
  reset: vi.fn(),
  capture: vi.fn(),
};

vi.mock("posthog-js", () => ({ default: posthogMock }));

let idleCallbacks: Array<() => void> = [];

async function loadAnalytics(key: string | undefined) {
  vi.resetModules();
  vi.stubEnv("NEXT_PUBLIC_POSTHOG_KEY", key ?? "");
  return (await import("./analytics")).default;
}

async function runIdle() {
  const cbs = idleCallbacks;
  idleCallbacks = [];
  for (const cb of cbs) cb();
  // Let the dynamic import + replay settle.
  await vi.dynamicImportSettled();
  await new Promise((r) => setTimeout(r, 0));
}

beforeEach(() => {
  for (const fn of Object.values(posthogMock)) fn.mockReset();
  idleCallbacks = [];
  vi.stubGlobal("requestIdleCallback", (cb: () => void) => {
    idleCallbacks.push(cb);
    return idleCallbacks.length;
  });
});

afterEach(() => {
  vi.unstubAllEnvs();
  vi.unstubAllGlobals();
});

describe("analytics (PostHog lazy loading)", () => {
  it("does not load PostHog until the browser is idle", async () => {
    const analytics = await loadAnalytics("phc_test");
    analytics.init();
    await new Promise((r) => setTimeout(r, 0));
    expect(posthogMock.init).not.toHaveBeenCalled();

    await runIdle();
    expect(posthogMock.init).toHaveBeenCalledTimes(1);
    expect(posthogMock.init).toHaveBeenCalledWith(
      "phc_test",
      expect.objectContaining({ disable_surveys: true }),
    );
  });

  it("queues calls made before load and replays them in order", async () => {
    const analytics = await loadAnalytics("phc_test");
    analytics.capture("early_event", { a: 1 });
    analytics.init();
    analytics.identify("u1", { name: "Test" });
    analytics.capture("second_event");
    analytics.reset();

    expect(posthogMock.capture).not.toHaveBeenCalled();
    await runIdle();

    expect(posthogMock.capture).toHaveBeenNthCalledWith(1, "early_event", { a: 1 });
    expect(posthogMock.identify).toHaveBeenCalledWith("u1", { name: "Test" });
    expect(posthogMock.capture).toHaveBeenNthCalledWith(2, "second_event", undefined);
    expect(posthogMock.reset).toHaveBeenCalledTimes(1);
    // Replay happens after init, in call order.
    const order = [
      posthogMock.init.mock.invocationCallOrder[0],
      posthogMock.capture.mock.invocationCallOrder[0],
      posthogMock.identify.mock.invocationCallOrder[0],
      posthogMock.capture.mock.invocationCallOrder[1],
      posthogMock.reset.mock.invocationCallOrder[0],
    ];
    expect(order).toEqual([...order].sort((a, b) => a - b));
  });

  it("calls straight through once loaded", async () => {
    const analytics = await loadAnalytics("phc_test");
    analytics.init();
    await runIdle();
    analytics.capture("after_load");
    expect(posthogMock.capture).toHaveBeenCalledWith("after_load", undefined);
  });

  it("only initialises once", async () => {
    const analytics = await loadAnalytics("phc_test");
    analytics.init();
    analytics.init();
    await runIdle();
    expect(posthogMock.init).toHaveBeenCalledTimes(1);
  });

  it("falls back to setTimeout when requestIdleCallback is unavailable", async () => {
    vi.stubGlobal("requestIdleCallback", undefined);
    vi.useFakeTimers();
    try {
      const analytics = await loadAnalytics("phc_test");
      analytics.init();
      expect(posthogMock.init).not.toHaveBeenCalled();
      await vi.runAllTimersAsync();
    } finally {
      vi.useRealTimers();
    }
    await vi.dynamicImportSettled();
    await new Promise((r) => setTimeout(r, 0));
    expect(posthogMock.init).toHaveBeenCalledTimes(1);
  });

  it("is a no-op when the PostHog key is unset", async () => {
    const analytics = await loadAnalytics(undefined);
    analytics.init();
    analytics.capture("ignored");
    analytics.identify("u1", {});
    analytics.reset();
    await runIdle();

    expect(idleCallbacks).toHaveLength(0);
    expect(posthogMock.init).not.toHaveBeenCalled();
    expect(posthogMock.capture).not.toHaveBeenCalled();
    expect(posthogMock.identify).not.toHaveBeenCalled();
    expect(posthogMock.reset).not.toHaveBeenCalled();
  });
});
