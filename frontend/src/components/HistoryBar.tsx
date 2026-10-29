import React, { useEffect, useRef, useState } from 'react';
import { api } from '../lib/api';
import { t } from '../i18n';
import { useUI, historyTabs } from '../lib/ui';
import { undoLast, useCanUndo, type UndoScope } from '../lib/undo';

const MIN_H = 60;

/** Bible history strip spanning the schedule and the browser; collapsible from its bottom edge. */
export default function HistoryBar({ saved }: { saved: Record<string, number> }) {
  const ui = useUI();
  const [open, setOpen] = useState<boolean>(() => (saved.historyOpen ?? 1) !== 0);
  const [height, setHeight] = useState<number>(() => Math.max(MIN_H, saved.historyHeight ?? 110));
  const [sel, setSel] = useState(-1);
  const hRef = useRef(height);
  hRef.current = height;
  const timer = useRef<number | null>(null);
  const listRef = useRef<HTMLDivElement>(null);
  const tab = ui.tab;
  const scope: UndoScope = tab === 'songs' ? 'history-songs' : 'history-bible';
  const canUndo = useCanUndo(scope);
  const entries: { label: string; right?: string; open: () => void; show: () => void }[] =
    tab === 'bible'
      ? ui.bibleHistory.map((h) => ({
          label: h.captionLong,
          open: () => ui.actions('bible')?.showFromSchedule?.({ type: 'bible', name: '', bible: h }),
          show: () => api.showBibleRef(h.verseIds),
        }))
      : tab === 'songs'
        ? ui.songHistory.map((h) => ({
            label: h.label,
            right: h.songbook,
            open: () => ui.actions('songs')?.showFromSchedule?.({ type: 'song', name: '', song: { id: h.id } }),
            show: () => api.showSong(h.id, 0),
          }))
        : [];
  const count = entries.length;

  const persist = (o: boolean, h: number) => {
    if (timer.current) window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => api.patchApp({ splitters: { historyOpen: o ? 1 : 0, historyHeight: h } }).catch(() => {}), 300);
  };
  useEffect(() => () => { if (timer.current) window.clearTimeout(timer.current); }, []);
  useEffect(() => {
    listRef.current?.scrollTo({ top: listRef.current.scrollHeight });
  }, [count, tab]);
  useEffect(() => {
    setSel(-1);
  }, [tab, count === 0]);
  // the first item shown in a tab opens the history panel
  const prevCount = useRef<Record<string, number>>({});
  useEffect(() => {
    if ((prevCount.current[tab] ?? 0) === 0 && count > 0) setOpen(true);
    prevCount.current[tab] = count;
  }, [tab, count]);

  const toggle = () => {
    setOpen((o) => {
      persist(!o, hRef.current);
      return !o;
    });
  };

  const onDown = (e: React.MouseEvent) => {
    e.preventDefault();
    const startY = e.clientY;
    const startH = hRef.current;
    const move = (ev: MouseEvent) => setHeight(Math.max(MIN_H, Math.min(startH + (startY - ev.clientY), window.innerHeight * 0.6)));
    const up = () => {
      window.removeEventListener('mousemove', move);
      window.removeEventListener('mouseup', up);
      document.body.style.cursor = '';
      persist(true, hRef.current);
    };
    document.body.style.cursor = 'row-resize';
    window.addEventListener('mousemove', move);
    window.addEventListener('mouseup', up);
  };

  const pick = (i: number) => {
    setSel(i);
    entries[i]?.open();
  };

  // only the Bible and Songs tabs keep a history
  // ...and with nothing shown yet there is no history to display
  if (!historyTabs.includes(tab) || count === 0) return null;

  return (
    <div className="history-bar">
      {open && (
        <>
          <div className="history-grip" onMouseDown={onDown} />
          <div className="history-header">
            <span style={{ fontWeight: 600 }}>{t('History')}</span>
            <span className="grow" />
            {canUndo && (
              <button className="tbtn-s" onClick={() => undoLast(scope)} title={t('Undo')}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 14l-4 -4l4 -4" /><path d="M5 10h11a4 4 0 1 1 0 8h-1" /></svg>
              </button>
            )}
            <button className="tbtn-s" disabled={count === 0} onClick={() => ui.clearHistory(tab)} title={t('Clear History')}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M19 20h-10.5l-4.21 -4.3a1 1 0 0 1 0 -1.41l10 -10a1 1 0 0 1 1.41 0l5 5a1 1 0 0 1 0 1.41l-9.2 9.3" /><path d="M18 13.3l-6.3 -6.3" /></svg>
            </button>
          </div>
          <div ref={listRef} className="list hover-items" tabIndex={0} style={{ height }} title={tab === 'bible' ? t('This list contains verses that were sent to be shown') : t('This list contains songs that were sent to be shown')}>
            {entries.map((h, i) => (
              <div key={i} className={`item ${i === sel ? 'sel' : ''}`} style={{ whiteSpace: 'nowrap', position: 'relative' }} onClick={() => pick(i)} onDoubleClick={() => h.show()}>
                <button
                  className="sched-del"
                  title={t('Remove from History')}
                  aria-label={t('Remove from History')}
                  draggable={false}
                  onClick={(e) => { e.stopPropagation(); setSel((s) => (s === i ? -1 : s > i ? s - 1 : s)); ui.removeHistory(tab, i); }}
                  onDoubleClick={(e) => e.stopPropagation()}
                >
                  <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
                </button>
                {h.right ? (
                  <div style={{ display: 'flex', gap: 12 }}>
                    <span style={{ flex: 1, minWidth: 0, overflow: 'hidden', textOverflow: 'ellipsis' }}>{h.label}</span>
                    <span className="muted" style={{ flex: '0 0 auto' }}>{h.right}</span>
                  </div>
                ) : <div style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{h.label}</div>}
              </div>
            ))}
          </div>
        </>
      )}
      <div className="history-edge" onClick={toggle} title={open ? t('Hide history') : t('Show history')}>
        <span>{open ? '⌄' : '⌃'}</span>
        {!open && <span>{t('History')}</span>}
      </div>
    </div>
  );
}
