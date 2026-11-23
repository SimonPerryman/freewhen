import { useId, type InputHTMLAttributes, type ReactNode, type Ref } from "react";
import CharCounter, { isAtLimit } from "@/components/CharCounter/CharCounter";
import styles from "./TextField.module.css";

export interface TextFieldProps extends Omit<InputHTMLAttributes<HTMLInputElement>, "children"> {
  label: ReactNode;
  // Keep the label for assistive tech but don't render it visually (for
  // fields whose purpose is obvious from the surrounding heading).
  hideLabel?: boolean;
  hint?: ReactNode;
  // Shown below the input with role="alert"; also sets aria-invalid.
  error?: ReactNode;
  // Show a CharCounter under the field (needs a string `value` and `maxLength`).
  counter?: boolean;
  // Class for the outer wrapper (layout), not the input.
  className?: string;
  ref?: Ref<HTMLInputElement>;
}

export default function TextField({
  label,
  hideLabel = false,
  hint,
  error,
  counter = false,
  className,
  id,
  type = "text",
  ref,
  "aria-describedby": describedByProp,
  ...rest
}: TextFieldProps) {
  const autoId = useId();
  const inputId = id ?? `${autoId}-input`;
  const hintId = hint ? `${inputId}-hint` : undefined;
  const errorId = error ? `${inputId}-error` : undefined;
  const count = counter ? String(rest.value ?? "").length : 0;
  const atLimit = counter && rest.maxLength !== undefined && isAtLimit(count, rest.maxLength);
  const describedBy = [describedByProp, hintId, errorId].filter(Boolean).join(" ") || undefined;

  return (
    <div className={[styles.field, className].filter(Boolean).join(" ")}>
      <label htmlFor={inputId} className={hideLabel ? styles.visuallyHidden : styles.label}>
        {label}
      </label>
      <input
        ref={ref}
        id={inputId}
        type={type}
        className={atLimit ? `${styles.input} ${styles.atLimit}` : styles.input}
        aria-invalid={error ? true : undefined}
        aria-describedby={describedBy}
        {...rest}
      />
      {hint ? (
        <p id={hintId} className={styles.hint}>
          {hint}
        </p>
      ) : null}
      {counter && rest.maxLength !== undefined ? (
        <div className={styles.counterRow}>
          <CharCounter count={count} max={rest.maxLength} />
        </div>
      ) : null}
      {error ? (
        <p id={errorId} className={styles.error} role="alert">
          {error}
        </p>
      ) : null}
    </div>
  );
}
