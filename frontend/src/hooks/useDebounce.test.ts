import { renderHook, act } from '@testing-library/react';
import { describe, expect, it, vi } from 'vitest';
import { useDebounce } from './useDebounce';

describe('useDebounce hook', () => {
  it('returns initial value immediately', () => {
    const { result } = renderHook(() => useDebounce('initial', 200));
    expect(result.current).toBe('initial');
  });

  it('debounces updates according to delay', () => {
    vi.useFakeTimers();
    const { result, rerender } = renderHook(({ val }) => useDebounce(val, 300), {
      initialProps: { val: 'first' },
    });

    expect(result.current).toBe('first');

    rerender({ val: 'second' });
    expect(result.current).toBe('first');

    act(() => {
      vi.advanceTimersByTime(299);
    });
    expect(result.current).toBe('first');

    act(() => {
      vi.advanceTimersByTime(1);
    });
    expect(result.current).toBe('second');

    vi.useRealTimers();
  });
});
