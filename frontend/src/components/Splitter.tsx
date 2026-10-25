import React, { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../lib/api';

interface Props {
  id: string;
  vertical?: boolean;
  initial?: number; // fraction of first pane
  min?: number; // px
  /** The first pane's own minimum, when it is not the same as the second's. */
  minFirst?: number;
  children: [React.ReactNode, React.ReactNode];
  className?: string;
  saved?: Record<string, number>;
  /** Keep the second pane at a fixed pixel size when the splitter itself is resized (the first pane gives way). */
  fixedSecond?: boolean;
  /** The second pane does not grow past this many px when the splitter itself is resized (or past the size the user dragged it to). */
  maxSecond?: number;
}

// The panes share the space in the ratio of their flex-grow. The grows must add up to at least 1: when they add up to
// less, a browser hands out only that part of the free space, and a pane held back by its maximum leaves a gap.
const grow = (f: number) => Math.round(f * 1000);

/** Two pane resizable splitter; the first pane fraction is persisted. */
export default function Splitter({ id, vertical, initial = 0.5, min = 60, minFirst = min, children, className, saved, fixedSecond, maxSecond }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [frac, setFrac] = useState<number>(() => {
    const v = saved?.[id];
    return v !== undefined && v > 0.02 && v < 0.98 ? v : initial;
  });
  const [secondPx, setSecondPx] = useState<number | null>(() => {
    const v = saved?.[id + 'Px'];
    return v !== undefined && v > 20 ? v : null;
  });
  // the size the user dragged the second pane to (saved apart from the other sizes: those are also made by the layout itself)
  const [dragPx, setDragPx] = useState<number | null>(() => {
    const v = saved?.[id + 'Drag'];
    return v !== undefined && v > 20 ? v : null;
  });
  // first layout: derive the pixel size from the saved / initial fraction
  useEffect(() => {
    if (fixedSecond && secondPx === null && ref.current) {
      const total = vertical ? ref.current.clientHeight : ref.current.clientWidth;
      setSecondPx(Math.max(min, Math.round(total * (1 - frac))));
    }
  }, [fixedSecond, secondPx, vertical, frac, min]);
  // a splitter that keeps proportions starts where the user last saw it: when its second pane was kept at a pixel size
  // before (that size is saved), the fraction is what that size is now
  useEffect(() => {
    const px = saved?.[id + 'Px'];
    if (fixedSecond || !px || !ref.current) return;
    const total = vertical ? ref.current.clientHeight : ref.current.clientWidth;
    if (total > 2 * min) setFrac(Math.max(minFirst / total, Math.min(1 - min / total, 1 - px / total)));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);
  const dragging = useRef(false);
  const timer = useRef<number | null>(null);

  const onDown = useCallback(
    (e: React.MouseEvent) => {
      e.preventDefault();
      dragging.current = true;
      const el = ref.current!;
      const rect = el.getBoundingClientRect();
      const move = (ev: MouseEvent) => {
        if (!dragging.current) return;
        const total = vertical ? rect.height : rect.width;
        const pos = vertical ? ev.clientY - rect.top : ev.clientX - rect.left;
        let f = pos / total;
        f = Math.max(minFirst / total, Math.min(1 - min / total, f));
        setFrac(f);
        const px = Math.round(Math.max(min, Math.min(total - minFirst, total - pos)));
        setSecondPx(px);
        setDragPx(px);
      };
      const up = () => {
        dragging.current = false;
        window.removeEventListener('mousemove', move);
        window.removeEventListener('mouseup', up);
        document.body.style.cursor = '';
      };
      document.body.style.cursor = vertical ? 'row-resize' : 'col-resize';
      window.addEventListener('mousemove', move);
      window.addEventListener('mouseup', up);
    },
    [vertical, min, minFirst],
  );

  useEffect(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      const sizes: Record<string, number> = { [id]: frac };
      if (fixedSecond && secondPx) sizes[id + 'Px'] = secondPx;
      if (maxSecond && dragPx) sizes[id + 'Drag'] = dragPx;
      api.patchApp({ splitters: sizes }).catch(() => {});
    }, 500);
  }, [frac, id, fixedSecond, maxSecond, secondPx, dragPx]);
  // what the user dragged it to counts as allowed: the limit is for the window being resized, not for the hand
  const cap = maxSecond ? Math.max(maxSecond, dragPx ?? 0) : undefined;

  return (
    <div ref={ref} className={`splitter ${vertical ? 'v' : ''} ${className || ''}`}>
      <div className="pane" style={fixedSecond ? { flex: '1 1 0px', minWidth: vertical ? undefined : minFirst, minHeight: vertical ? minFirst : undefined } : { flex: `${grow(frac)} 1 0px`, minWidth: vertical ? undefined : minFirst, minHeight: vertical ? minFirst : undefined }}>
        {children[0]}
      </div>
      <div className="gutter" onMouseDown={onDown} />
      <div className="pane" style={fixedSecond && secondPx ? { flex: `0 1 ${secondPx}px`, minWidth: vertical ? undefined : min, minHeight: vertical ? min : undefined } : { flex: `${grow(1 - frac)} 1 0px`, minWidth: vertical ? undefined : min, minHeight: vertical ? min : undefined, maxWidth: vertical ? undefined : cap, maxHeight: vertical ? cap : undefined }}>
        {children[1]}
      </div>
    </div>
  );
}
