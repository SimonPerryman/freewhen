import Link from "next/link";
import styles from "./status-page.module.css";

export default function NotFound() {
  return (
    <div className={styles.page}>
      <div className={styles.inner}>
        <h1 className={styles.title}>Page not found</h1>
        <p className={styles.message}>The page you&apos;re looking for doesn&apos;t exist.</p>
        <Link href="/" className={styles.action}>
          Back to home
        </Link>
      </div>
    </div>
  );
}
