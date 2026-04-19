"use client";

import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import styles from "./Header.module.css";

export default function Header() {
  const pathname = usePathname();
  const { user, isLoading, checkAuth, signOut } = useAuth();
  const [signingOut, setSigningOut] = useState(false);

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  // Don't render on the login page
  if (pathname === "/login") return null;

  async function handleSignOut() {
    setSigningOut(true);
    await signOut();
    setSigningOut(false);
  }

  return (
    <header className={styles.header}>
      <a href="/" className={styles.logo}>
        FreeWhen
      </a>
      <div className={styles.actions}>
        {isLoading ? null : user ? (
          <div className={styles.userArea}>
            <span className={styles.userName}>{user.name || user.email}</span>
            <button
              type="button"
              className={styles.signOutButton}
              onClick={handleSignOut}
              disabled={signingOut}
            >
              Sign out
            </button>
          </div>
        ) : (
          <a
            href={`/login?return_url=${encodeURIComponent(pathname)}`}
            className={styles.signInLink}
          >
            Sign in
          </a>
        )}
      </div>
    </header>
  );
}
