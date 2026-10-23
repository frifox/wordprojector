import { useCallback, useEffect, useRef, useState } from 'react';
import { useStore } from './store';
import { onLanguageChange } from '../i18n';

/** Re-renders the component when the UI language changes. */
export function useT() {
  const [, set] = useState(0);
  useEffect(() => onLanguageChange(() => set((n) => n + 1)), []);
}

/** Fetches data and refetches whenever the given data set is invalidated. */
export function useData<T>(what: string, loader: () => Promise<T>, deps: unknown[] = []): [T | null, () => void] {
  const { dataVersion } = useStore();
  const [data, setData] = useState<T | null>(null);
  const ver = dataVersion[what] || 0;
  const all = dataVersion.all || 0;
  const reload = useCallback(() => {
    loader().then(setData).catch((e) => console.error(what, e));
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [what, ...deps]);
  useEffect(() => {
    reload();
  }, [reload, ver, all]);
  return [data, reload];
}

/** Global keyboard shortcut registration. Handler returns true when it handled the event. */
export function useShortcuts(handler: (e: KeyboardEvent) => boolean | void) {
  const ref = useRef(handler);
  ref.current = handler;
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      if (ref.current(e)) {
        e.preventDefault();
        e.stopPropagation();
      }
    };
    window.addEventListener('keydown', h);
    return () => window.removeEventListener('keydown', h);
  }, []);
}

export const isMac = navigator.platform.toLowerCase().includes('mac') || navigator.userAgent.includes('Macintosh');

/** Returns true when the "command" modifier (Ctrl, or Cmd on macOS) is held. */
export function cmdKey(e: KeyboardEvent | React.KeyboardEvent): boolean {
  return isMac ? e.metaKey : e.ctrlKey;
}

export function isTyping(e: KeyboardEvent): boolean {
  const t = e.target as HTMLElement | null;
  if (!t) return false;
  const tag = t.tagName;
  return tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || t.isContentEditable;
}

export function shortcutLabel(s: string): string {
  return isMac ? s.replace('Ctrl+', '⌘').replace('Shift+', '⇧').replace('Alt+', '⌥').replace('Del', '⌫') : s;
}

/** Simple debounced value. */
export function useDebounce<T>(value: T, ms: number): T {
  const [v, setV] = useState(value);
  useEffect(() => {
    const t = window.setTimeout(() => setV(value), ms);
    return () => window.clearTimeout(t);
  }, [value, ms]);
  return v;
}
