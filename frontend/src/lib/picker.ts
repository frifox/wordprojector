import { useSyncExternalStore } from 'react';

// Number of native file dialogs that are open (or were closed a moment ago). The Esc key that closes such a dialog can
// reach the page too, so the dialogs inside the app must not take it as their own Esc.
let open = 0;
// Number of native file dialogs that are open right now; the buttons that open one wait for it.
let active = 0;
const listeners = new Set<() => void>();

export const filePickerOpen = () => open > 0;
/** True while a native file dialog is open: another one must not be started. */
export const pickerActive = () => active > 0;

const changed = () => listeners.forEach((l) => l());

/** Re-renders the component while a native file dialog is open (use it to disable the buttons that open one). */
export function usePickerActive(): boolean {
  return useSyncExternalStore((l) => { listeners.add(l); return () => { listeners.delete(l); }; }, pickerActive);
}

export async function trackPicker<T>(work: Promise<T>): Promise<T> {
  open++;
  active++;
  changed();
  try {
    return await work;
  } finally {
    active--;
    changed();
    window.setTimeout(() => { open--; }, 500);
  }
}
