import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, waitFor } from "@testing-library/react";

const replaceMock = vi.fn();
const pushMock = vi.fn();
const checkAuthMock = vi.fn();
let search = new URLSearchParams();

vi.mock("next/navigation", () => ({
  useRouter: () => ({ replace: replaceMock, push: pushMock }),
  useSearchParams: () => search,
}));

vi.mock("@/components/AuthProvider/AuthProvider", () => ({
  useAuth: () => ({ user: null, checkAuth: checkAuthMock }),
}));

import LoginPage from "./page";

beforeEach(() => {
  replaceMock.mockReset();
  pushMock.mockReset();
  checkAuthMock.mockReset();
});

function renderWith(returnUrl: string | null) {
  search = new URLSearchParams();
  if (returnUrl !== null) search.set("return_url", returnUrl);
  return render(<LoginPage />);
}

describe("LoginPage return_url handling", () => {
  it("redirects an authenticated user to a same-origin return_url", async () => {
    checkAuthMock.mockResolvedValue({ id: "u1" });
    renderWith("/calendars");
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/calendars"));
  });

  it.each(["//evil.com", "/\\evil.com", "https://evil.com", "javascript:alert(1)", "/\t/evil.com"])(
    "redirects an authenticated user to / for hostile return_url %j",
    async (hostile) => {
      checkAuthMock.mockResolvedValue({ id: "u1" });
      renderWith(hostile);
      await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/"));
      expect(replaceMock).not.toHaveBeenCalledWith(hostile);
    },
  );

  it("defaults to / when return_url is missing", async () => {
    checkAuthMock.mockResolvedValue({ id: "u1" });
    renderWith(null);
    await waitFor(() => expect(replaceMock).toHaveBeenCalledWith("/"));
  });

  it("does not redirect when unauthenticated, and passes only the safe path to Google sign-in", async () => {
    checkAuthMock.mockResolvedValue(null);
    renderWith("//evil.com");
    await waitFor(() => expect(checkAuthMock).toHaveBeenCalled());
    expect(replaceMock).not.toHaveBeenCalled();
    const link = screen.getByRole("link", { name: /continue with google/i });
    expect(link.getAttribute("href")).toMatch(/return_url=%2F$/);
  });
});
