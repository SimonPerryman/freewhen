import { describe, expect, it, vi } from "vitest";
import { act, render, screen } from "@testing-library/react";
import { renderToString } from "react-dom/server";
import { hydrateRoot } from "react-dom/client";
import { localToday } from "./localDate";
import { ServerTodayProvider, useToday } from "./useToday";

function Today() {
  return <span>{useToday()}</span>;
}

describe("useToday", () => {
  it("returns the local date in a client render (provider ignored)", () => {
    render(
      <ServerTodayProvider value="2000-01-01">
        <Today />
      </ServerTodayProvider>,
    );
    expect(screen.getByText(localToday())).toBeInTheDocument();
  });

  it("server-renders the server's date, hydrates cleanly, then switches to the local date", async () => {
    const element = (
      <ServerTodayProvider value="2000-01-01">
        <Today />
      </ServerTodayProvider>
    );
    const html = renderToString(element);
    expect(html).toContain("2000-01-01");

    const container = document.createElement("div");
    container.innerHTML = html;
    document.body.appendChild(container);
    const onRecoverableError = vi.fn();
    let root: ReturnType<typeof hydrateRoot> | null = null;
    await act(async () => {
      root = hydrateRoot(container, element, { onRecoverableError });
    });

    expect(onRecoverableError).not.toHaveBeenCalled();
    expect(container.textContent).toBe(localToday());
    act(() => root!.unmount());
    container.remove();
  });

  it("server-renders the local date without a provider", () => {
    expect(renderToString(<Today />)).toContain(localToday());
  });
});
