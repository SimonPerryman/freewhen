type Level = "debug" | "info" | "warn" | "error";

const order: Record<Level, number> = { debug: 10, info: 20, warn: 30, error: 40 };

function parseLevel(raw: string | undefined): Level {
  switch (raw) {
    case "debug":
    case "info":
    case "warn":
    case "error":
      return raw;
    default:
      return "info";
  }
}

const threshold = order[parseLevel(process.env.NEXT_PUBLIC_LOG_LEVEL)];

const noop = () => {};

const gate = (level: Level, fn: (...args: unknown[]) => void) =>
  order[level] >= threshold ? fn : noop;

const log = {
  debug: gate("debug", console.debug.bind(console)),
  info: gate("info", console.info.bind(console)),
  warn: gate("warn", console.warn.bind(console)),
  error: gate("error", console.error.bind(console)),
};

export default log;
