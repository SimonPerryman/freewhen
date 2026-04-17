import Link from "next/link";

export default function NotFound() {
  return (
    <div
      style={{
        display: "flex",
        minHeight: "100vh",
        alignItems: "center",
        justifyContent: "center",
        padding: 24,
        fontFamily: "var(--font-geist-sans), sans-serif",
        backgroundColor: "var(--page-bg)",
        color: "var(--text-primary)",
      }}
    >
      <div
        style={{
          width: "100%",
          maxWidth: 480,
          textAlign: "center",
        }}
      >
        <h1
          style={{
            margin: "0 0 8px",
            fontSize: 28,
            fontWeight: 600,
            letterSpacing: "-0.5px",
          }}
        >
          Page not found
        </h1>
        <p
          style={{
            margin: "0 0 28px",
            fontSize: 15,
            color: "var(--text-secondary)",
          }}
        >
          The page you're looking for doesn't exist.
        </p>
        <Link
          href="/"
          style={{
            display: "inline-block",
            padding: "12px 16px",
            fontSize: 15,
            fontWeight: 500,
            fontFamily: "inherit",
            color: "var(--accent-text)",
            background: "var(--accent)",
            borderRadius: 8,
            textDecoration: "none",
          }}
        >
          Back to home
        </Link>
      </div>
    </div>
  );
}
