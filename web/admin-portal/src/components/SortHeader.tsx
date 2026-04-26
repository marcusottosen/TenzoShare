import type { SortState } from '../hooks/useSort';

interface SortHeaderProps<T extends string> {
  label: string;
  /** The sort key this header controls. */
  sortKey: T;
  /** Sort state object from useSortState. */
  sort: SortState<T>;
}

/**
 * A <th> that shows sort direction arrows and toggles sort on click.
 * Use alongside useSortState() for a consistent sort experience across all tables.
 */
export function SortHeader<T extends string>({ label, sortKey, sort }: SortHeaderProps<T>) {
  const active = sort.sortKey === sortKey;
  return (
    <th className="th-sort" onClick={() => sort.toggle(sortKey)}>
      {label}
      <span className={`sort-icon${active ? ' sort-icon-active' : ''}`}>
        {active ? (sort.sortDir === 'asc' ? '↑' : '↓') : '↕'}
      </span>
    </th>
  );
}
