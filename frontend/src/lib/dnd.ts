import type { ScheduleItem } from './types';
import { api } from './api';

/** Drag payload type used to drop verses and songs onto the Schedule list. */
export const SCHEDULE_MIME = 'application/x-wordprojector-schedule-item';

export function startScheduleDrag(e: React.DragEvent, item: ScheduleItem) {
  e.dataTransfer.effectAllowed = 'copy';
  e.dataTransfer.setData(SCHEDULE_MIME, JSON.stringify(item));
  e.dataTransfer.setData('text/plain', item.name || item.bible?.caption || '');
}

export function hasScheduleDrag(e: React.DragEvent): boolean {
  return Array.from(e.dataTransfer.types).includes(SCHEDULE_MIME);
}

export function readScheduleDrag(e: React.DragEvent): ScheduleItem | null {
  try {
    const raw = e.dataTransfer.getData(SCHEDULE_MIME);
    return raw ? (JSON.parse(raw) as ScheduleItem) : null;
  } catch {
    return null;
  }
}

/** A slide show dragged from the list carries no photos (they are fetched when it is dropped). */
export async function resolveScheduleItem(item: ScheduleItem): Promise<ScheduleItem | null> {
  if (item.type !== 'slideshow' || !item.slideshow || item.slideshow.slides.length || item.slideshow.id <= 0) return item;
  try {
    const ss = await api.slideshow(item.slideshow.id);
    return ss.slides.length ? { ...item, name: ss.name, slideshow: ss } : null;
  } catch {
    return null;
  }
}
