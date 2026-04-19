"use client";

import { createContext, useContext, useState, useCallback, type ReactNode } from "react";
import { getMe, logout as apiLogout, ApiError } from "@/lib/api";
import analytics from "@/lib/analytics";
import type { User } from "@/lib/types";

interface AuthContextValue {
  user: User | null;
  isLoading: boolean;
  isAuthenticated: boolean;
  checkAuth: () => Promise<User | null>;
  clearAuth: () => void;
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

export default function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [checked, setChecked] = useState(false);

  const checkAuth = useCallback(async (): Promise<User | null> => {
    if (checked && user) return user;
    setIsLoading(true);
    try {
      const me = await getMe();
      setUser(me);
      analytics.identify(me.id, { email: me.email });
      return me;
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        setUser(null);
        return null;
      }
      setUser(null);
      return null;
    } finally {
      setIsLoading(false);
      setChecked(true);
    }
  }, [checked, user]);

  const clearAuth = useCallback(() => {
    setUser(null);
    setChecked(false);
  }, []);

  const signOut = useCallback(async () => {
    try {
      await apiLogout();
    } finally {
      setUser(null);
      setChecked(false);
      analytics.reset();
    }
  }, []);

  return (
    <AuthContext.Provider
      value={{
        user,
        isLoading,
        isAuthenticated: !!user,
        checkAuth,
        clearAuth,
        signOut,
      }}
    >
      {children}
    </AuthContext.Provider>
  );
}
