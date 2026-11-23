"use client";

import { usePathname } from "next/navigation";
import { useAuth } from "@/components/AuthProvider/AuthProvider";
import { ColorSchemeIconButton } from "@/components/ColorSchemeToggle/ColorSchemeToggle";
import UserMenu from "@/components/UserMenu/UserMenu";
import styles from "./Header.module.css";

export default function Header() {
  const pathname = usePathname();
  const { user, isLoading } = useAuth();

  // On the sign-in screens the bar carries only the logo: a "Sign in" link
  // there is a dead end, and the signed-in nav is irrelevant mid-auth.
  const onAuthScreen = pathname === "/login" || pathname.startsWith("/login/");

  return (
    <header className={styles.header}>
      <a href="/" className={`${styles.logo} wordmark`} aria-label="FreeWhen home">
        Free<span className="serif-accent">When</span>
      </a>
      <nav aria-label="Primary" className={styles.actions}>
        {onAuthScreen ? (
          <ColorSchemeIconButton />
        ) : isLoading ? null : user ? (
          <>
            <a href="/calendars" className={styles.myCalendarsLink}>
              My calendars
            </a>
            <UserMenu />
          </>
        ) : (
          <>
            <ColorSchemeIconButton />
            <a
              href={
                pathname === "/" ? "/login" : `/login?return_url=${encodeURIComponent(pathname)}`
              }
              className={styles.signInLink}
            >
              Sign in
            </a>
          </>
        )}
      </nav>
    </header>
  );
}
