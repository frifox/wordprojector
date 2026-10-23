import React, { useCallback, useEffect, useRef, useState } from 'react';
import { api } from '../lib/api';
import { t } from '../i18n';
import { hasScheduleDrag, readScheduleDrag, resolveScheduleItem } from '../lib/dnd';

const MIN_W = 200;
const CLOSE_W = 100;
const EDGE_W = 14;

/**
 * Collapsible, resizable left sidebar.
 * - The left edge strip toggles open/closed.
 * - The right edge can be dragged; width is clamped to MIN_W, and dragging
 *   below CLOSE_W closes the sidebar.
 */
export default function Sidebar({ saved, children }: { saved: Record<string, number>; children: React.ReactNode }) {
  const [open, setOpen] = useState<boolean>(() => (saved.scheduleOpen ?? 0) !== 0);
  const [width, setWidth] = useState<number>(() => Math.max(MIN_W, saved.scheduleWidth ?? 260));
  const [dragging, setDragging] = useState(false);
  const [closing, setClosing] = useState(false); // drag is past the close threshold
  const [dropHot, setDropHot] = useState(false); // a verse or song is being dragged over the collapsed edge
  const widthRef = useRef(width);
  widthRef.current = width;
  const timer = useRef<number | null>(null);
  const hoverTimer = useRef<number | null>(null); // opens the pane after hovering a drag over the collapsed edge
  const stopHover = () => {
    if (hoverTimer.current) window.clearTimeout(hoverTimer.current);
    hoverTimer.current = null;
  };

  const persist = useCallback((o: boolean, w: number) => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => {
      api.patchApp({ splitters: { scheduleOpen: o ? 1 : 0, scheduleWidth: w } }).catch(() => {});
    }, 300);
  }, []);

  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current); stopHover(); }, []);

  const toggle = () => {
    const next = !open;
    setOpen(next);
    persist(next, widthRef.current);
  };

  const onDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const startX = e.clientX;
    const startW = widthRef.current;
    setDragging(true);
    let last = startW;
    let willClose = false;
    const move = (ev: MouseEvent) => {
      const raw = startW + (ev.clientX - startX);
      last = raw;
      willClose = raw < CLOSE_W;
      setClosing(willClose);
      setWidth(Math.max(MIN_W, Math.min(raw, window.innerWidth * 0.6)));
    };
    const up = () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
      document.body.style.cursor = '';
      setDragging(false);
      setClosing(false);
      if (willClose || last < CLOSE_W) {
        setOpen(false);
        persist(false, startW < MIN_W ? MIN_W : startW);
        setWidth(Math.max(MIN_W, startW));
      } else {
        persist(true, widthRef.current);
      }
    };
    document.body.style.cursor = 'col-resize';
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };

  const shown = open && !(dragging && closing);
  return (
    <div className={`sidebar ${dragging ? "dragging" : ""}`} style={{ width: open ? width + EDGE_W : EDGE_W, opacity: dragging && closing ? 0.5 : 1 }}>
      <div
        className={`sidebar-edge ${dropHot ? 'drop-hot' : ''}`}
        onClick={toggle}
        title={open ? t('Hide schedule') : t('Show schedule')}
        // while collapsed, dropping a verse or song on the edge adds it to the schedule and opens the pane
        onDragOver={(e) => { if (!open && hasScheduleDrag(e)) { e.preventDefault(); e.dataTransfer.dropEffect = 'copy';
            setDropHot(true);
            if (!hoverTimer.current) {
              hoverTimer.current = window.setTimeout(() => {
                hoverTimer.current = null;
                setDropHot(false);
                setOpen(true);
                persist(true, widthRef.current);
              }, 1000);
            }
          } }}
        onDragLeave={() => { setDropHot(false); stopHover(); }}
        onDrop={async (e) => {
          setDropHot(false);
          stopHover();
          if (open || !hasScheduleDrag(e)) return;
          e.preventDefault();
          const dropped = readScheduleDrag(e);
          const item = dropped && await resolveScheduleItem(dropped);
          if (!item) return;
          await api.scheduleAdd(item);
          setOpen(true);
          persist(true, widthRef.current);
        }}
      >
        <span>{open ? '‹' : '›'}</span>
        {!open && <span className="sidebar-edge-label">{t('Schedule')}</span>}
      </div>
      {open && (
        <>
          <div className="sidebar-body" style={{ display: shown || dragging ? 'flex' : 'none' }}>{children}</div>
          <div className="sidebar-grip" onMouseDown={onDown} />
        </>
      )}
    </div>
  );
}
