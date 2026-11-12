import { describe, it, expect, vi, beforeEach, afterEach } from "vitest";
import { act, render, screen, waitFor } from "@testing-library/react";
import { useEffect } from "react";

vi.mock("@/lib/analytics", () => ({
  default: { init: vi.fn(), identify: vi.fn(), reset: vi.fn(), capture: vi.fn() },
}));

const logMock = vi.hoisted(() => ({
  debug: vi.fn(),
  info: vi.fn(),
  warn: vi.fn(),
  error: vi.fn(),
}));
vi.mock("@/lib/log", () => ({ default: logMock }));

const getMeMock = vi.fn();
const logoutMock = vi.fn();

vi.mock("@/lib/api", async () => {
  const actual = await vi.importActual<typeof import("@/lib/api")>("@/lib/api");
  return {
    ...actual,
    getMe: () => getMeMock(),
    logout: () => logoutMock(),
  };
});

import AuthProvider, { useAuth } from "./AuthProvider";
import { ApiError } from "@/lib/api";

const testUser = { id: "u1", email: "test@example.com", name: "Test" };

let consoleErrorSpy: ReturnType<typeof vi.spyOn>;

beforeEach(() => {
  getMeMock.mockReset();
  logoutMock.mockReset();
  for (const fn of Object.values(logMock)) fn.mockReset();
  consoleErrorSpy = vi.spyOn(console, "error");
});

afterEach(() => {
  consoleErrorSpy.mockRestore();
});

// A consumer that, like the old Header/home page, also calls checkAuth on mount.
function EagerConsumer({ label }: { label: string }) {
  const { user, isLoading, checkAuth } = useAuth();
  useEffect(() => {
    void checkAuth();
  }, [checkAuth]);
  return <p>{`${label}:${isLoading ? "loading" : (user?.name ?? "signed-out")}`}</p>;
}

describe("AuthProvider", () => {
  it("issues a single /auth/me for many consumers", async () => {
    getMeMock.mockResolvedValue(testUser);
    render(
      <AuthProvider>
        <EagerConsumer label="a" />
        <EagerConsumer label="b" />
        <EagerConsumer label="c" />
      </AuthProvider>,
    );

    expect(await screen.findByText("a:Test")).toBeInTheDocument();
    expect(screen.getByText("b:Test")).toBeInTheDocument();
    expect(screen.getByText("c:Test")).toBeInTheDocument();
    expect(getMeMock).toHaveBeenCalledTimes(1);
  });

  it("returns the cached result to later checkAuth calls", async () => {
    getMeMock.mockResolvedValue(testUser);
    let check: (() => Promise<unknown>) | undefined;
    function Grab() {
      check = useAuth().checkAuth;
      return null;
    }
    render(
      <AuthProvider>
        <Grab />
      </AuthProvider>,
    );
    await waitFor(() => expect(getMeMock).toHaveBeenCalledTimes(1));
    await expect(check!()).resolves.toEqual(testUser);
    await expect(check!()).resolves.toEqual(testUser);
    expect(getMeMock).toHaveBeenCalledTimes(1);
  });

  it("treats a 401 as signed out without logging an error, and caches it", async () => {
    getMeMock.mockRejectedValue(new ApiError(401, "not authenticated"));
    let check: (() => Promise<unknown>) | undefined;
    function Grab() {
      check = useAuth().checkAuth;
      return null;
    }
    render(
      <AuthProvider>
        <EagerConsumer label="a" />
        <EagerConsumer label="b" />
        <Grab />
      </AuthProvider>,
    );

    expect(await screen.findByText("a:signed-out")).toBeInTheDocument();
    expect(screen.getByText("b:signed-out")).toBeInTheDocument();
    await expect(check!()).resolves.toBeNull();
    expect(getMeMock).toHaveBeenCalledTimes(1);
    expect(consoleErrorSpy).not.toHaveBeenCalled();
    expect(logMock.error).not.toHaveBeenCalled();
    expect(logMock.warn).not.toHaveBeenCalled();
  });

  it("warns (not errors) on unexpected failures", async () => {
    getMeMock.mockRejectedValue(new ApiError(500, "boom"));
    render(
      <AuthProvider>
        <EagerConsumer label="a" />
      </AuthProvider>,
    );
    expect(await screen.findByText("a:signed-out")).toBeInTheDocument();
    expect(logMock.warn).toHaveBeenCalledTimes(1);
    expect(logMock.error).not.toHaveBeenCalled();
  });

  it("signOut clears the user without re-requesting /auth/me", async () => {
    getMeMock.mockResolvedValue(testUser);
    logoutMock.mockResolvedValue(undefined);
    let signOut: (() => Promise<void>) | undefined;
    function Grab() {
      signOut = useAuth().signOut;
      return null;
    }
    render(
      <AuthProvider>
        <EagerConsumer label="a" />
        <Grab />
      </AuthProvider>,
    );
    expect(await screen.findByText("a:Test")).toBeInTheDocument();

    await act(async () => {
      await signOut!();
    });

    expect(screen.getByText("a:signed-out")).toBeInTheDocument();
    expect(getMeMock).toHaveBeenCalledTimes(1);
  });
});
