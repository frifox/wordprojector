import { useSyncExternalStore } from 'react';
import { api } from './api';
import type { ScheduleItem } from './types';

/** What the undo key takes back, most recent last. Each removal belongs to the panel it was made in, which has an Undo
 *  button of its own: 'schedule', 'slides', 'history-bible', 'history-songs'. */
export type UndoScope = 'schedule' | 'slides' | 'history-bible' | 'history-songs';
const stack: { scope: UndoScope; undo: () => void | Promise<unknown> }[] = [];
const listeners = new Set<() => void>();
const LIMIT = 50;

export function pushUndo(scope: UndoScope, undo: () => void | Promise<unknown>) {
  stack.push({ scope, undo });
  if (stack.length > LIMIT) stack.shift();
  listeners.forEach((l) => l());
}

/** Takes back the last removal (of a panel when given). False when there is nothing to take back. */
export function undoLast(scope?: UndoScope): boolean {
  let i = stack.length - 1;
  while (i >= 0 && scope && stack[i].scope !== scope) i--;
  if (i < 0) return false;
  const [e] = stack.splice(i, 1);
  listeners.forEach((l) => l());
  Promise.resolve(e.undo()).catch(() => {});
  return true;
}

/** Whether a removal of the panel can be taken back. */
export function useCanUndo(scope: UndoScope): boolean {
  return useSyncExternalStore(
    (l) => { listeners.add(l); return () => { listeners.delete(l); }; },
    () => stack.some((e) => e.scope === scope),
  );
}

/** Puts removed items back into the Schedule, each at the place it had. */
async function restoreScheduled(removed: { item: ScheduleItem; index: number }[]) {
  for (const r of removed) {
    await api.scheduleAdd(r.item);
    const n = (await api.schedule()).items.length;
    if (r.index < n - 1) await api.scheduleMove(n - 1, r.index);
  }
}

export function removeScheduled(items: ScheduleItem[], index: number) {
  const item = items[index];
  if (!item) return;
  api.scheduleRemove(index);
  pushUndo('schedule', () => restoreScheduled([{ item, index }]));
}

export function clearScheduled(items: ScheduleItem[]) {
  if (!items.length) return;
  const all = items.map((item, index) => ({ item, index }));
  api.scheduleClear();
  pushUndo('schedule', () => restoreScheduled(all));
}
