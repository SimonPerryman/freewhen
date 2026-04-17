const isDev = process.env.NODE_ENV !== "production";

const noop = () => {};

const log = {
  info: isDev ? console.info.bind(console) : noop,
  warn: isDev ? console.warn.bind(console) : noop,
  error: console.error.bind(console),
};

export default log;
