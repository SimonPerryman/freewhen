// Base URL of the Go API. Read once from NEXT_PUBLIC_API_URL (inlined at
// build time); defaults to the local dev backend.
export const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL ?? "http://localhost:8080";
