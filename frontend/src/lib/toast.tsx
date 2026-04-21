"use client";

import { createContext, useCallback, useContext, useMemo, useRef, useState } from "react";
import Toast, { type ToastAction } from "@/components/Toast/Toast";

interface ShowToastInput {
  message: string;
  action?: ToastAction;
  durationMs?: number;
}

interface ToastContextValue {
  showToast: (input: ShowToastInput) => void;
  dismiss: () => void;
}

const DEFAULT_DURATION_MS = 6000;

const ToastContext = createContext<ToastContextValue | null>(null);

interface ActiveToast {
  id: number;
  message: string;
  action?: ToastAction;
  durationMs: number;
}

export function ToastProvider({ children }: { children: React.ReactNode }) {
  const [toast, setToast] = useState<ActiveToast | null>(null);
  const nextIdRef = useRef(0);

  const dismiss = useCallback(() => {
    setToast(null);
  }, []);

  const showToast = useCallback((input: ShowToastInput) => {
    nextIdRef.current += 1;
    setToast({
      id: nextIdRef.current,
      message: input.message,
      action: input.action,
      durationMs: input.durationMs ?? DEFAULT_DURATION_MS,
    });
  }, []);

  const value = useMemo<ToastContextValue>(() => ({ showToast, dismiss }), [showToast, dismiss]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      {toast ? (
        <Toast
          key={toast.id}
          toastKey={toast.id}
          message={toast.message}
          action={toast.action}
          durationMs={toast.durationMs}
          onDismiss={dismiss}
        />
      ) : null}
    </ToastContext.Provider>
  );
}

export function useToast(): ToastContextValue {
  const ctx = useContext(ToastContext);
  if (!ctx) {
    throw new Error("useToast must be used inside a ToastProvider");
  }
  return ctx;
}
