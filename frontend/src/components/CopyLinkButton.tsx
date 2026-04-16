"use client";

import { useState, useEffect } from "react";
import styles from "./CopyLinkButton.module.css";

export default function CopyLinkButton() {
  const [supported, setSupported] = useState(false);
  const [copied, setCopied] = useState(false);

  useEffect(() => {
    setSupported(typeof navigator !== "undefined" && !!navigator.clipboard);
  }, []);

  if (!supported) return null;

  async function handleClick() {
    try {
      await navigator.clipboard.writeText(window.location.href);
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    } catch {
      // clipboard write failed — ignore silently
    }
  }

  return (
    <button type="button" className={styles.button} onClick={handleClick}>
      {copied ? "Copied!" : "Copy link"}
    </button>
  );
}
