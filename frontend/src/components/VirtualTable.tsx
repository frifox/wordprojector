import { useEffect, useRef, useState } from 'react';

// Column widths dragged by the user. Kept only in memory, so they reset when the app restarts.
const dragged = new Map<string, Record<string, number>>();

export interface Column {
  key: string;
  label: string;
  width: number; // 0 = flexible
  sort?: number; // 0 none, 1 asc, -1 desc
}

interface Props<T> {
  rows: T[];
  rowKey: (r: T) => number | string;
  columns: readonly Column[];
  selectedKey: number | string;
  onSelect: (r: T) => void;
  onActivate?: (r: T) => void;
  onSort?: (key: string) => void;
  render: (r: T, key: string) => React.ReactNode;
  className?: string;
  rowHeight?: number;
  /** Called on a printable key press (not prevented, so the character still reaches whatever gets focus). */
  onType?: (e: React.KeyboardEvent) => void;
  /** Lets the user drag column borders; sizes are remembered for this id until the app restarts. */
  resizeId?: string;
  /** Makes rows draggable; fill the drag payload in here. */
  onRowDragStart?: (r: T, e: React.DragEvent) => void;
  /** Extra key handler, called for keys the table does not use itself (e.g. Left / Right). */
  onKeyExtra?: (e: React.KeyboardEvent) => void;
  /** Receives the focusable table element. */
  tableRef?: React.MutableRefObject<HTMLDivElement | null>;
}

/** Windowed table for large song lists. */
export default function VirtualTable<T>({ rows, rowKey, columns, selectedKey, onSelect, onActivate, onSort, render, className, rowHeight = 22, onType, resizeId, onRowDragStart, onKeyExtra, tableRef }: Props<T>) {
  const ref = useRef<HTMLDivElement>(null);
  const [widths, setWidthsState] = useState<Record<string, number> | undefined>(() => (resizeId ? dragged.get(resizeId) : undefined));
  const setWidths = (w: Record<string, number>) => {
    if (resizeId) dragged.set(resizeId, w);
    setWidthsState(w);
  };
  // saved sizes only apply when they cover exactly the columns shown now
  const usable = widths && columns.every((c) => widths[c.key] > 0) ? widths : undefined;
  const startResize = (i: number) => (e: React.MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    const ths = ref.current?.querySelectorAll('th');
    if (!ths) return;
    const base: Record<string, number> = {};
    columns.forEach((c, k) => { base[c.key] = ths[k].getBoundingClientRect().width; });
    const a = columns[i].key;
    const b = columns[i + 1].key;
    const startX = e.clientX;
    const MIN = 30;
    const move = (ev: MouseEvent) => {
      const dx = Math.max(MIN - base[a], Math.min(base[b] - MIN, ev.clientX - startX));
      setWidths({ ...base, [a]: base[a] + dx, [b]: base[b] - dx });
    };
    const up = () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
      document.body.style.cursor = '';
    };
    document.body.style.cursor = 'col-resize';
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };
  const [scrollTop, setScrollTop] = useState(0);
  useEffect(() => { if (tableRef) tableRef.current = ref.current; });
  const [height, setHeight] = useState(300);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setHeight(el.clientHeight));
    ro.observe(el);
    setHeight(el.clientHeight);
    return () => ro.disconnect();
  }, []);
  // keep selection visible
  useEffect(() => {
    const idx = rows.findIndex((r) => rowKey(r) === selectedKey);
    const el = ref.current;
    if (idx < 0 || !el) return;
    const top = idx * rowHeight + 24;
    if (top < el.scrollTop + 24) el.scrollTop = top - 24;
    else if (top + rowHeight > el.scrollTop + el.clientHeight) el.scrollTop = top + rowHeight - el.clientHeight;
  }, [selectedKey, rows, rowKey, rowHeight, height]);

  const start = Math.max(0, Math.floor(scrollTop / rowHeight) - 5);
  const end = Math.min(rows.length, Math.ceil((scrollTop + height) / rowHeight) + 5);
  const visible = rows.slice(start, end);
  const onKey = (e: React.KeyboardEvent) => {
    onKeyExtra?.(e);
    if (onType && e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey) onType(e);
    const idx = rows.findIndex((r) => rowKey(r) === selectedKey);
    if (e.key === 'ArrowDown' && idx < rows.length - 1) {
      onSelect(rows[idx + 1]);
      e.preventDefault();
    } else if (e.key === 'ArrowUp' && idx > 0) {
      onSelect(rows[idx - 1]);
      e.preventDefault();
    } else if (e.key === 'Enter' && idx >= 0) {
      onActivate?.(rows[idx]);
    } else if (e.key === 'PageDown' || e.key === 'PageUp') {
      const page = Math.max(1, Math.floor(height / rowHeight) - 1);
      const n = e.key === 'PageDown' ? Math.min(rows.length - 1, idx + page) : Math.max(0, idx - page);
      if (rows[n]) onSelect(rows[n]);
      e.preventDefault();
      e.stopPropagation();
    }
  };
  return (
    <div ref={ref} className={`table ${className || ''}`} tabIndex={0} onScroll={(e) => setScrollTop(e.currentTarget.scrollTop)} onKeyDown={onKey}>
      <table>
        <colgroup>
          {columns.map((c) => (
            <col key={c.key} style={usable ? { width: usable[c.key] } : c.width ? { width: c.width } : undefined} />
          ))}
        </colgroup>
        <thead>
          <tr>
            {columns.map((c, i) => (
              <th key={c.key} onClick={() => onSort?.(c.key)}>
                {c.label}
                {resizeId && i < columns.length - 1 && (
                  <span
                    onMouseDown={startResize(i)}
                    onClick={(e) => e.stopPropagation()}
                    style={{ position: 'absolute', top: 0, right: -3, bottom: 0, width: 7, cursor: 'col-resize', zIndex: 2 }}
                  />
                )}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {start > 0 && (
            <tr style={{ height: start * rowHeight }}>
              <td colSpan={columns.length} style={{ padding: 0 }} />
            </tr>
          )}
          {visible.map((r) => {
            const k = rowKey(r);
            return (
              <tr key={k} className={k === selectedKey ? 'sel' : ''} style={{ height: rowHeight }} draggable={!!onRowDragStart} onDragStart={onRowDragStart ? (e) => onRowDragStart(r, e) : undefined} onClick={() => onSelect(r)} onDoubleClick={() => onActivate?.(r)}>
                {columns.map((c) => (
                  <td key={c.key}>{render(r, c.key)}</td>
                ))}
              </tr>
            );
          })}
          {end < rows.length && (
            <tr style={{ height: (rows.length - end) * rowHeight }}>
              <td colSpan={columns.length} style={{ padding: 0 }} />
            </tr>
          )}
        </tbody>
      </table>
    </div>
  );
}
