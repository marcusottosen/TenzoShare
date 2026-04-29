import { useState } from 'react';

export type SortDir = 'asc' | 'desc';

export interface SortState<T extends string> {
  sortKey: T;
  sortDir: SortDir;
  toggle: (key: T) => void;
}

/**
 * Manages a sortKey + sortDir pair.
 * Toggling the same key flips the direction; a new key resets direction to 'asc'.
 * Pass an optional `onSort` callback (e.g. `() => setPage(0)`) to run side-effects after any change.
 */
export function useSortState<T extends string>(
  initialKey: T,
  initialDir: SortDir = 'desc',
  onSort?: () => void,
): SortState<T> {
  const [sortKey, setSortKey] = useState<T>(initialKey);
  const [sortDir, setSortDir] = useState<SortDir>(initialDir);

  function toggle(key: T) {
    if (sortKey === key) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'));
    } else {
      setSortKey(key);
      setSortDir('asc');
    }
    onSort?.();
  }

  return { sortKey, sortDir, toggle };
}
