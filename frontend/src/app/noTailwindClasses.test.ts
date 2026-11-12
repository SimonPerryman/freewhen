import { readdirSync, readFileSync, statSync } from "node:fs";
import path from "node:path";
import { describe, it, expect } from "vitest";

// The project styles with CSS Modules + tokens; there is no Tailwind. A
// Tailwind utility string in a className renders as completely unstyled
// markup (it happened on the magic-link callback page), so fail fast if one
// sneaks back in.
const SRC = path.resolve(__dirname, "..");

const UTILITY =
  /^(?:[a-z]+:)*-?(?:[mp][trblxy]?-(?:\d|auto|px)|(?:text|bg|border|rounded|font|leading|tracking|gap|space-[xy]|max-w|min-w|min-h|max-h|w|h|items|justify|grid-cols|shadow|opacity|z)-[\w./[\]-]+$|(?:flex|grid|hidden|block|inline-flex|underline|uppercase|truncate)$)/;

function sourceFiles(dir: string): string[] {
  return readdirSync(dir).flatMap((name) => {
    const full = path.join(dir, name);
    if (statSync(full).isDirectory()) return sourceFiles(full);
    return name.endsWith(".tsx") && !name.endsWith(".test.tsx") ? [full] : [];
  });
}

// Literal class strings only: className="…" and className={`…`} (the
// static parts of the template).
function literalClassTokens(source: string): string[] {
  const out: string[] = [];
  const re = /className=(?:"([^"]*)"|\{`([^`]*)`\})/g;
  for (const m of source.matchAll(re)) {
    const raw = (m[1] ?? m[2] ?? "").replace(/\$\{[^}]*\}/g, " ");
    out.push(...raw.split(/\s+/).filter(Boolean));
  }
  return out;
}

describe("no Tailwind utility classes", () => {
  it("flags Tailwind-style tokens", () => {
    for (const t of ["mx-auto", "px-4", "text-sm", "bg-neutral-900", "disabled:opacity-50"]) {
      expect(UTILITY.test(t), t).toBe(true);
    }
    for (const t of ["pill", "skip-link", "serif-accent"]) {
      expect(UTILITY.test(t), t).toBe(false);
    }
  });

  it("finds none in src", () => {
    const offenders = sourceFiles(SRC).flatMap((file) =>
      literalClassTokens(readFileSync(file, "utf8"))
        .filter((t) => UTILITY.test(t))
        .map((t) => `${path.relative(SRC, file)}: ${t}`),
    );
    expect(offenders).toEqual([]);
  });
});
