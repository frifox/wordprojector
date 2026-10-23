import React, { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../lib/api';

interface Props {
  id: string;
  vertical?: boolean;
  initial?: number; // fraction of first pane
  min?: number; // px
  children: [React.ReactNode, React.ReactNode];
  className?: string;
  saved?: Record<string, number>;
  /** Keep the second pane at a fixed pixel size when the splitter itself is resized (the first pane gives way). */
  fixedSecond?: boolean;
}

/** Two pane resizable splitter; the first pane fraction is persisted. */
export default function Splitter({ id, vertical, initial = 0.5, min = 60, children, className, saved, fixedSecond }: Props) {
  const ref = useRef<HTMLDivElement>(null);
  const [frac, setFrac] = useState<number>(() => {
    const v = saved?.[id];
    return v !== undefined && v > 0.02 && v < 0.98 ? v : initial;
  });
  const [secondPx, setSecondPx] = useState<number | null>(() => {
    const v = saved?.[id + 'Px'];
    return v !== undefined && v > 20 ? v : null;
  });
  // first layout: derive the pixel size from the saved / initial fraction
  useEffect(() => {
    if (fixedSecond && secondPx === null && ref.current) {
      const total = vertical ? ref.current.clientHeight : ref.current.clientWidth;
      setSecondPx(Math.max(min, Math.round(total * (1 - frac))));
    }
  }, [fixedSecond, secondPx, vertical, frac, min]);
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
        const minF = min / total;
        f = Math.max(minF, Math.min(1 - minF, f));
        setFrac(f);
        setSecondPx(Math.round(Math.max(min, Math.min(total - min, total - pos))));
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
    [vertical, min],
  );

  useEffect(() => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      api.patchApp({ splitters: fixedSecond && secondPx ? { [id]: frac, [id + 'Px']: secondPx } : { [id]: frac } }).catch(() => {});
    }, 500);
  }, [frac, id, fixedSecond, secondPx]);

  return (
    <div ref={ref} className={`splitter ${vertical ? 'v' : ''} ${className || ''}`}>
      <div className="pane" style={fixedSecond ? { flex: '1 1 0px', minWidth: vertical ? undefined : min, minHeight: vertical ? min : undefined } : { flex: `${frac} 1 0px` }}>
        {children[0]}
      </div>
      <div className="gutter" onMouseDown={onDown} />
      <div className="pane" style={fixedSecond && secondPx ? { flex: `0 1 ${secondPx}px`, minWidth: vertical ? undefined : min, minHeight: vertical ? min : undefined } : { flex: `${1 - frac} 1 0px` }}>
        {children[1]}
      </div>
    </div>
  );
}
