"use client";

import { Suspense } from "react";
import { useSearchParams } from "next/navigation";
import styles from "../page.module.css";
import checkStyles from "./page.module.css";

export default function CheckEmailPage() {
  return (
    <Suspense>
      <CheckEmailContent />
    </Suspense>
  );
}

function CheckEmailContent() {
  const searchParams = useSearchParams();
  const email = searchParams.get("email") ?? "your email";

  return (
    <div className={styles.page}>
      <main className={styles.container}>
        <h1 className={styles.heading}>Check your email</h1>
        <p className={styles.subheading}>
          We sent a sign-in link to <strong className={checkStyles.email}>{email}</strong>. Click
          the link to sign in.
        </p>
        <p className={checkStyles.hint}>
          The link expires in 15 minutes. If you don't see it, check your spam folder.
        </p>
      </main>
    </div>
  );
}
