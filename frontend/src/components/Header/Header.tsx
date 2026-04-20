"use client";

import { useEffect } from "react";
import { usePathname } from "next/navigation";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import UserMenu from "@/components/UserMenu/UserMenu";
import styles from "./Header.module.css";

export default function Header() {
  const pathname = usePathname();
  const { user, isLoading, checkAuth } = useAuth();

  useEffect(() => {
    checkAuth();
  }, [checkAuth]);

  if (pathname === "/login") return null;

  return (
    <header className={styles.header}>
      <a href="/" className={styles.logo}>
        FreeWhen
      </a>
      <div className={styles.actions}>
        {isLoading ? null : user ? (
          <>
            <a href="/calendars" className={styles.myCalendarsLink}>
              My calendars
            </a>
            <UserMenu />
          </>
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
