import styles from "./page.module.css";

// Shown while the server fetches the calendar (route-level Suspense
// fallback, so the shell streams immediately even on an API cold start) and
// by CalendarPageClient while a client-side load is in flight.
export default function CalendarPageLoading() {
  return (
    <div className={styles.page}>
      <div className={styles.container}>
        <p className={styles.loading}>Loading…</p>
      </div>
    </div>
  );
}
