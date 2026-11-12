import { describe, it, expect, vi, beforeEach } from "vitest";
import { fireEvent, render, screen, waitFor } from "@testing-library/react";

let search = new URLSearchParams();
const verifyMock = vi.fn();

vi.mock("next/navigation", () => ({
  useSearchParams: () => search,
}));

vi.mock("@/lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("@/lib/api")>();
  return { ...actual, verifyMagicLink: (token: string) => verifyMock(token) };
});

import { ApiError } from "@/lib/api";
import MagicLinkCallbackPage from "./page";

beforeEach(() => {
  verifyMock.mockReset();
  search = new URLSearchParams();
});

describe("MagicLinkCallbackPage", () => {
  it("shows the invalid-link state when the token is missing", () => {
    render(<MagicLinkCallbackPage />);
    expect(screen.getByRole("heading", { name: "Invalid link" })).toBeInTheDocument();
    expect(screen.queryByRole("button")).not.toBeInTheDocument();
  });

  it("renders Continue as the shared primary, full-width Button", () => {
    search.set("token", "tok");
    render(<MagicLinkCallbackPage />);
    expect(screen.getByRole("heading", { name: "Sign in to FreeWhen" })).toBeInTheDocument();
    const button = screen.getByRole("button", { name: "Continue" });
    expect(button.className).toContain("button");
    expect(button.className).toContain("primary");
    expect(button.className).toContain("fullWidth");
  });

  it("disables the button and marks it busy while verifying", async () => {
    search.set("token", "tok");
    verifyMock.mockReturnValue(new Promise(() => {}));
    render(<MagicLinkCallbackPage />);
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    const busy = await screen.findByRole("button", { name: /Signing you in/ });
    expect(busy).toBeDisabled();
    expect(busy).toHaveAttribute("aria-busy", "true");
    expect(verifyMock).toHaveBeenCalledWith("tok");
  });

  it("shows the error as an alert and re-enables the button", async () => {
    search.set("token", "tok");
    verifyMock.mockRejectedValue(new ApiError(400, "Link expired"));
    render(<MagicLinkCallbackPage />);
    fireEvent.click(screen.getByRole("button", { name: "Continue" }));
    expect(await screen.findByRole("alert")).toHaveTextContent("Link expired");
    await waitFor(() => expect(screen.getByRole("button", { name: "Continue" })).toBeEnabled());
  });
});
