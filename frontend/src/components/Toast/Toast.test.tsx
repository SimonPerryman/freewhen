import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, fireEvent, render, screen } from "@testing-library/react";
import { ToastProvider, useToast } from "@/lib/toast";

function Harness({ onReady }: { onReady: (ctx: ReturnType<typeof useToast>) => void }) {
  const ctx = useToast();
  onReady(ctx);
  return null;
}

function setup() {
  let ctx!: ReturnType<typeof useToast>;
  render(
    <ToastProvider>
      <Harness onReady={(c) => (ctx = c)} />
    </ToastProvider>,
  );
  return () => ctx;
}

describe("Toast", () => {
  beforeEach(() => {
    vi.useFakeTimers();
  });

  afterEach(() => {
    vi.useRealTimers();
  });

  it("shows a toast on showToast()", () => {
    const getCtx = setup();
    expect(screen.queryByTestId("toast")).not.toBeInTheDocument();

    act(() => {
      getCtx().showToast({ message: "hello" });
    });

    expect(screen.getByTestId("toast")).toBeInTheDocument();
    expect(screen.getByText("hello")).toBeInTheDocument();
  });

  it("a second showToast replaces the first (single slot)", () => {
    const getCtx = setup();

    act(() => {
      getCtx().showToast({ message: "first" });
    });
    expect(screen.getByText("first")).toBeInTheDocument();

    act(() => {
      getCtx().showToast({ message: "second" });
    });
    expect(screen.queryByText("first")).not.toBeInTheDocument();
    expect(screen.getByText("second")).toBeInTheDocument();
    expect(screen.getAllByTestId("toast")).toHaveLength(1);
  });

  it("dismiss() removes the toast", () => {
    const getCtx = setup();
    act(() => {
      getCtx().showToast({ message: "hi" });
    });
    expect(screen.getByTestId("toast")).toBeInTheDocument();

    act(() => {
      getCtx().dismiss();
    });
    expect(screen.queryByTestId("toast")).not.toBeInTheDocument();
  });

  it("auto-dismisses after durationMs", () => {
    const getCtx = setup();
    act(() => {
      getCtx().showToast({ message: "bye", durationMs: 1000 });
    });
    expect(screen.getByTestId("toast")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(999);
    });
    expect(screen.getByTestId("toast")).toBeInTheDocument();

    act(() => {
      vi.advanceTimersByTime(2);
    });
    expect(screen.queryByTestId("toast")).not.toBeInTheDocument();
  });

  it("action click fires callback and dismisses the toast", () => {
    const getCtx = setup();
    const onClick = vi.fn();

    act(() => {
      getCtx().showToast({
        message: "cleared",
        action: { label: "Undo", onClick },
        durationMs: 5000,
      });
    });

    act(() => {
      fireEvent.click(screen.getByRole("button", { name: "Undo" }));
    });
    expect(onClick).toHaveBeenCalledTimes(1);
    expect(screen.queryByTestId("toast")).not.toBeInTheDocument();
  });

  it("hover pauses the auto-dismiss timer", () => {
    const getCtx = setup();

    act(() => {
      getCtx().showToast({ message: "hoverme", durationMs: 1000 });
    });

    const toast = screen.getByTestId("toast");
    act(() => {
      fireEvent.mouseEnter(toast);
    });

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByTestId("toast")).toBeInTheDocument();

    act(() => {
      fireEvent.mouseLeave(toast);
    });
    act(() => {
      vi.advanceTimersByTime(1100);
    });
    expect(screen.queryByTestId("toast")).not.toBeInTheDocument();
  });
});
