// safeReturnPath guards every client-side navigation to a caller-supplied
// return URL (`?return_url=`, magic-link verify response, etc). Only
// same-origin relative paths survive; anything else collapses to "/".
//
// Rules:
//   - must start with a single "/" (rejects "https://…", "javascript:…",
//     bare hosts)
//   - second char must not be "/" or "\" — "//evil.com" and "/\evil.com" are
//     treated by browsers as protocol-relative URLs to another host
//   - no "\" anywhere, no whitespace or control characters — the URL parser
//     strips tabs/newlines and normalises "\" to "/", which is how
//     "/\t/evil.com" style tricks smuggle a second slash past naive checks
//
// Percent-encoded slashes ("/%2F%2Fevil.com") are allowed: the browser keeps
// them as path content on the current origin.
// oxlint-disable-next-line no-control-regex -- matching control chars is the point
const FORBIDDEN = /[\u0000- \u007f-\u009f\\\s]/;

export function safeReturnPath(raw: string | null | undefined): string {
  if (typeof raw !== "string" || raw === "") return "/";
  if (raw[0] !== "/") return "/";
  if (raw[1] === "/" || raw[1] === "\\") return "/";
  if (FORBIDDEN.test(raw)) return "/";
  return raw;
}
