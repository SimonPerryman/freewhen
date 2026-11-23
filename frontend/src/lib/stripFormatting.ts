// Strip inline markdown emphasis from a calendar name so a copy doesn't carry
// literal asterisks or backticks. Only paired markers are removed, so names
// like "snake_case" or "5 * 3" survive.
export function stripFormatting(name: string): string {
  return name
    .replace(/\[([^\]]+)\]\([^)]*\)/g, "$1")
    .replace(/(\*\*|__)(.+?)\1/g, "$2")
    .replace(/\*(.+?)\*/g, "$1")
    .replace(/(?<!\w)_(.+?)_(?!\w)/g, "$1")
    .replace(/~~(.+?)~~/g, "$1")
    .replace(/`([^`]+)`/g, "$1")
    .replace(/^\s*(#{1,6}\s+|>\s+)/, "")
    .replace(/\s+/g, " ")
    .trim();
}
