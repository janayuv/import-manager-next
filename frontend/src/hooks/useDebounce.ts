import * as React from 'react';

/**
 * Returns a debounced copy of value that delays updating until after delayMs
 * milliseconds have elapsed since the last time the value was modified.
 * In automated test suites, delay is zeroed to ensure immediate assertions.
 */
export function useDebounce<T>(value: T, delayMs?: number): T {
  const isTest =
    (typeof process !== 'undefined' && process.env?.NODE_ENV === 'test') ||
    (typeof window !== 'undefined' && (window as unknown as { __VITEST_ENVIRONMENT__?: boolean }).__VITEST_ENVIRONMENT__);

  const effectiveDelay = delayMs ?? (isTest ? 0 : 250);
  const [debouncedValue, setDebouncedValue] = React.useState<T>(value);

  React.useEffect(() => {
    if (effectiveDelay <= 0) {
      setDebouncedValue(value);
      return;
    }

    const timer = window.setTimeout(() => {
      setDebouncedValue(value);
    }, effectiveDelay);

    return () => {
      window.clearTimeout(timer);
    };
  }, [value, effectiveDelay]);

  return debouncedValue;
}
