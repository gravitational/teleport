/**
 * ALLOWED_DURATION_SYNTAX defines simplified Go duration string syntax matcher.
 */
const ALLOWED_DURATION_SYNTAX =
  /^[+]?(?:0|(?:(?:\d+(?:\.\d*)?|\.\d+)(?:ns|us|µs|μs|ms|s|m|h))+)$/;

export const isZeroDuration = (value: string) =>
  ALLOWED_DURATION_SYNTAX.test(value) && !/[1-9]/.test(value);
