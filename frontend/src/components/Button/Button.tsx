import type { ButtonHTMLAttributes, Ref } from "react";
import styles from "./Button.module.css";

export type ButtonVariant = "primary" | "secondary" | "ghost" | "danger";
export type ButtonSize = "sm" | "md" | "lg";

interface StyleOptions {
  variant?: ButtonVariant;
  size?: ButtonSize;
  fullWidth?: boolean;
  className?: string;
}

// Class list for anything that should look like a Button — e.g. a Next
// <Link> styled as a call to action. Prefer <Button> for real buttons.
export function buttonClassName({
  variant = "secondary",
  size = "md",
  fullWidth = false,
  className,
}: StyleOptions = {}): string {
  return [
    styles.button,
    styles[variant],
    styles[size],
    fullWidth ? styles.fullWidth : "",
    className,
  ]
    .filter(Boolean)
    .join(" ");
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, StyleOptions {
  // Disables the button, sets aria-busy and shows a spinner before the label.
  // Callers keep control of the label text (e.g. "Saving…").
  loading?: boolean;
  ref?: Ref<HTMLButtonElement>;
}

export default function Button({
  variant,
  size,
  fullWidth,
  className,
  loading = false,
  disabled,
  type = "button",
  children,
  ref,
  ...rest
}: ButtonProps) {
  return (
    <button
      ref={ref}
      type={type}
      className={buttonClassName({ variant, size, fullWidth, className })}
      disabled={disabled || loading}
      aria-busy={loading || undefined}
      {...rest}
    >
      {loading ? <span className={styles.spinner} aria-hidden="true" /> : null}
      {children}
    </button>
  );
}
