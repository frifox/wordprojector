import { useEffect, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { focusLive } from '../lib/focus';
import { removeScheduled, clearScheduled, undoLast, useCanUndo } from '../lib/undo';
import { t } from '../i18n';
import { Icon, useUI, type TabName } from '../lib/ui';
import type { ScheduleItem } from '../lib/types';
import { hasScheduleDrag, readScheduleDrag, resolveScheduleItem } from '../lib/dnd';

const icons: Record<ScheduleItem['type'], string> = { bible: 'book.png', song: 'song_tab.png', slideshow: 'photo.png', media: 'video.png', announce: 'announce.png' };
const tabOf: Record<ScheduleItem['type'], TabName> = { bible: 'bible', song: 'songs', slideshow: 'pictures', media: 'media', announce: 'announce' };

export default function SchedulePanel({ selected, setSelected, onAdd }: { selected: number; setSelected: (i: number) => void; onAdd: () => void }) {
  const { schedule } = useStore();
  const canUndo = useCanUndo('schedule');
  const ui = useUI();
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (selected >= schedule.items.length) setSelected(schedule.items.length - 1);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [schedule.items.length]);

  // drag to re-order: the other rows slide aside to preview the drop position.
  // Hit-testing uses the rows' resting positions (taken at drag start) so the sliding rows can't feed back into it.
  const itemRefs = useRef<(HTMLDivElement | null)[]>([]);
  const slots = useRef<{ top: number; height: number }[]>([]);
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const [overIdx, setOverIdx] = useState<number | null>(null);
  const slotAt = (clientY: number): number => {
    const list = slots.current;
    if (!list.length) return 0;
    if (clientY < list[0].top) return 0;
    const k = list.findIndex((sl) => clientY >= sl.top && clientY < sl.top + sl.height);
    return k < 0 ? list.length - 1 : k;
  };
  const movedOrder = (from: number, target: number) => {
    const order = schedule.items.map((_, i) => i).filter((i) => i !== from);
    order.splice(target, 0, from);
    return order;
  };
  const slideOffset = (i: number): number => {
    if (dragFrom === null || overIdx === null || dragFrom === overIdx) return 0;
    const order = movedOrder(dragFrom, overIdx);
    let y = 0;
    for (const k of order) {
      if (k === i) break;
      y += slots.current[k]?.height ?? 0;
    }
    return y - slots.current.slice(0, i).reduce((sum, sl) => sum + sl.height, 0);
  };
  // verses and songs dragged in from the browsers are inserted at the line shown under the pointer
  const [insertAt, setInsertAt] = useState<number | null>(null);
  const insertionIndex = (clientY: number): number => {
    const rows = itemRefs.current.slice(0, schedule.items.length);
    const k = rows.findIndex((el) => el && clientY < el.getBoundingClientRect().top + el.getBoundingClientRect().height / 2);
    return k < 0 ? rows.length : k;
  };
  const endDrag = () => { setDragFrom(null); setOverIdx(null); };
  const dropAt = (clientY: number) => {
    const from = dragFrom;
    const to = slotAt(clientY);
    endDrag();
    if (from === null || from === to) return;
    api.scheduleMove(from, to).then(() => setSelected(to));
  };

  const select = (i: number) => {
    setSelected(i);
    const it = schedule.items[i];
    if (!it) return;
    ui.setTab(tabOf[it.type]);
    // let the tab mount, then preview
    setTimeout(() => ui.actions(tabOf[it.type])?.showFromSchedule?.(it), 0);
  };

  return (
    <div className="col fill" style={{ gap: 4 }}>
      <div className="row" style={{ gap: 2 }}>
        <span style={{ fontWeight: 600 }}>{t('Schedule')}</span>
        <span className="grow" />
        {canUndo && <button className="tbtn-s" title={t('Undo')} onClick={() => undoLast('schedule')}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 14l-4 -4l4 -4" /><path d="M5 10h11a4 4 0 1 1 0 8h-1" /></svg></button>}
        <button className="tbtn-s" title={t('Add to Schedule')} onClick={onAdd}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5l0 14" /><path d="M5 12l14 0" /></svg></button>
        <button className="tbtn-s" title={t('Remove from Schedule')} disabled={selected < 0} onClick={() => removeScheduled(schedule.items, selected)}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12l14 0" /></svg></button>
        <button className="tbtn-s" title={t('Clear Schedule')} disabled={schedule.items.length === 0} onClick={() => clearScheduled(schedule.items)}><svg viewBox="0 0 24 24" aria-hidden="true"><path d="M19 20h-10.5l-4.21 -4.3a1 1 0 0 1 0 -1.41l10 -10a1 1 0 0 1 1.41 0l5 5a1 1 0 0 1 0 1.41l-9.2 9.3" /><path d="M18 13.3l-6.3 -6.3" /></svg></button>
      </div>
      <div
        ref={ref}
        className="list grow"
        style={schedule.items.length === 0 && insertAt !== null ? { outline: '2px dashed var(--accent)', outlineOffset: -2 } : undefined}
        tabIndex={0}
        onDragOver={(e) => {
          if (dragFrom === null && hasScheduleDrag(e)) {
            e.preventDefault();
            e.dataTransfer.dropEffect = 'copy';
            const at = insertionIndex(e.clientY);
            setInsertAt((o) => (o === at ? o : at));
            return;
          }
          if (dragFrom === null) return;
          e.preventDefault();
          const target = slotAt(e.clientY);
          setOverIdx((o) => (o === target ? o : target));
        }}
        onDragLeave={(e) => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setInsertAt(null); }}
        onDrop={async (e) => {
          e.preventDefault();
          if (dragFrom === null && hasScheduleDrag(e)) {
            const dropped = readScheduleDrag(e);
            const at = insertionIndex(e.clientY);
            setInsertAt(null);
            const item = dropped && await resolveScheduleItem(dropped);
            if (!item) return;
            const n = schedule.items.length;
            await api.scheduleAdd(item);
            if (at < n) await api.scheduleMove(n, at);
            setSelected(Math.min(at, n));
            return;
          }
          dropAt(e.clientY);
        }}
        onKeyDown={(e) => {
          if (e.key === 'ArrowDown' && selected < schedule.items.length - 1) {
            select(selected + 1);
            e.preventDefault();
          } else if (e.key === 'ArrowUp' && selected > 0) {
            select(selected - 1);
            e.preventDefault();
          } else if (e.key === 'Enter' && selected >= 0) {
            api.showSchedule(selected);
            focusLive();
          }
        }}
      >
        {schedule.items.map((it, i) => (
          <div
            key={i}
            ref={(el) => { itemRefs.current[i] = el; }}
            draggable
            onDragStart={(e) => {
              e.dataTransfer.effectAllowed = 'move';
              e.dataTransfer.setData('text/plain', String(i));
              slots.current = schedule.items.map((_, k) => {
                const r = itemRefs.current[k]!.getBoundingClientRect();
                return { top: r.top, height: r.height };
              });
              // the state change waits a tick so the browser snapshots the row before it fades
              setTimeout(() => { setDragFrom(i); setOverIdx(i); }, 0);
            }}
            onDragEnd={endDrag}
            className={`item ${i === selected ? 'sel' : ''}`}
            style={{
              display: 'flex', alignItems: 'center', gap: 6, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis',
              transform: dragFrom !== null ? `translateY(${slideOffset(i)}px)` : undefined,
              opacity: dragFrom === i ? 0.45 : 1,
              transition: dragFrom !== null ? 'transform 0.15s ease' : undefined,
              cursor: 'grab',
              position: 'relative',
              boxShadow: insertAt === i ? 'inset 0 2px 0 var(--accent)' : insertAt === schedule.items.length && i === schedule.items.length - 1 ? 'inset 0 -2px 0 var(--accent)' : undefined,
            }}
            onClick={() => select(i)}
            onDoubleClick={() => {
              select(i);
              api.showSchedule(i);
              focusLive();
            }}
          >
            <Icon name={icons[it.type]} />
            <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{it.name}</span>
            <button
              className="sched-del"
              title={t('Remove from Schedule')}
              aria-label={t('Remove from Schedule')}
              draggable={false}
              onClick={(e) => { e.stopPropagation(); if (i < selected) setSelected(selected - 1); removeScheduled(schedule.items, i); }}
              onDoubleClick={(e) => e.stopPropagation()}
            >
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
            </button>
          </div>
        ))}
      </div>
    </div>
  );
}
