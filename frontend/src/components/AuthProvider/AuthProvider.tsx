"use client";

import {
  createContext,
  useContext,
  useState,
  useCallback,
  useEffect,
  useMemo,
  useRef,
  type ReactNode,
} from "react";
import { getMe, logout as apiLogout, ApiError } from "@/lib/api";
import analytics from "@/lib/analytics";
import log from "@/lib/log";
import type { User } from "@/lib/types";

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  checkAuth: () => Promise<User | null>;
  clearAuth: () => void;
  setUser: (user: User) => void;
  signOut: () => Promise<void>;
}

const AuthContext = createContext<AuthContextValue | null>(null);

export function useAuth(): AuthContextValue {
  const ctx = useContext(AuthContext);
  if (!ctx) {
    throw new Error("useAuth must be used within an AuthProvider");
  }
  return ctx;
}

// AuthProvider is the single source of truth for "who is signed in". It
// checks /auth/me once on mount; consumers read `user`/`isLoading` from
// context. `checkAuth` is safe to call from anywhere: it returns the cached
// result (signed in *or* signed out) or joins the one in-flight request, so
// a page never triggers more than one /auth/me.
export default function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUserState] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(true);

  // Bumped whenever auth state is set or cleared locally, so a stale
  // in-flight /auth/me can't overwrite a newer answer.
  const generation = useRef(0);
  const inflight = useRef<Promise<User | null> | null>(null);
  const resolved = useRef<{ user: User | null } | null>(null);

  const checkAuth = useCallback((): Promise<User | null> => {
    if (resolved.current) return Promise.resolve(resolved.current.user);
    if (inflight.current) return inflight.current;

    const gen = generation.current;
    setIsLoading(true);
    const request = getMe()
      .then(
        (me): User | null => me,
        (err: unknown): User | null => {
          // 401 is the normal signed-out state, not an error.
          if (!(err instanceof ApiError && err.status === 401)) {
            log.warn("auth: /auth/me failed", { error: String(err) });
          }
          return null;
        },
      )
      .then((me) => {
        if (gen !== generation.current) {
          // Superseded by setUser/clearAuth/signOut; report current state.
          return resolved.current?.user ?? null;
        }
        resolved.current = { user: me };
        inflight.current = null;
        setUserState(me);
        setIsLoading(false);
        if (me) analytics.identify(me.id, { email: me.email });
        return me;
      });
    inflight.current = request;
    return request;
  }, []);

  useEffect(() => {
    void checkAuth();
  }, [checkAuth]);

  const setUser = useCallback((next: User) => {
    generation.current += 1;
    inflight.current = null;
    resolved.current = { user: next };
    setUserState(next);
    setIsLoading(false);
  }, []);

  const clearAuth = useCallback(() => {
    generation.current += 1;
    inflight.current = null;
    resolved.current = null;
    setUserState(null);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      generation.current += 1;
      inflight.current = null;
      resolved.current = { user: null };
      setUserState(null);
      setIsLoading(false);
      analytics.reset();
    }
  }, []);

  const value = useMemo(
    () => ({
      user,
      isLoading,
      isAuthenticated: !!user,
      checkAuth,
      clearAuth,
      setUser,
      signOut,
    }),
    [user, isLoading, checkAuth, clearAuth, setUser, signOut],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}
