import { isMac } from './hooks';

export interface HotkeyAction {
  id: string;
  label: string;
  /** the combination in the notation of `comboOf` */
  def: string;
}

/** The keys the user may change, in the sections of Settings > Hotkeys, and the keys they have unless changed. */
export const HOTKEY_GROUPS: { title: string; items: HotkeyAction[] }[] = [
  {
    title: 'Tabs',
    items: [
      { id: 'tabBible', label: 'Bible', def: 'F6' },
      { id: 'tabSongs', label: 'Songs', def: 'F7' },
      { id: 'tabSlides', label: 'Slides', def: 'F8' },
    ],
  },
  {
    title: 'Presentation',
    items: [
      { id: 'play', label: 'Play', def: 'F5' },
      { id: 'next', label: 'Next slide', def: 'PageDown' },
      { id: 'prev', label: 'Previous slide', def: 'PageUp' },
    ],
  },
  {
    title: 'Other',
    items: [{ id: 'print', label: 'Print', def: '' }],
  },
];

export const HOTKEYS: HotkeyAction[] = HOTKEY_GROUPS.flatMap((g) => g.items);

/** The keys that cannot be changed. */
const FIXED: Record<string, string> = {
  pause: 'Escape', // pressed again it turns the presentation off, like the Off button
  pauseOnly: 'Shift+Escape',
  stop: 'Delete', // turn off and clear the live list
  removeItem: 'Mod+Delete',
  settings: 'Mod+,',
  exit: 'Mod+Q',
};

/** Whether a key combination belongs to an action that cannot be changed. */
export function isReserved(combo: string): boolean {
  return Object.values(FIXED).includes(combo) || combo === 'Backspace' || combo === 'Mod+Backspace';
}

/** The key of each action: the user's choice, or the default. */
export function hotkeyMap(chosen?: Record<string, string>): Record<string, string> {
  const m: Record<string, string> = { ...FIXED };
  for (const h of HOTKEYS) m[h.id] = chosen && h.id in chosen ? chosen[h.id] : h.def;
  return m;
}

const MODIFIER_KEYS = ['Control', 'Shift', 'Alt', 'Meta', 'AltGraph', 'CapsLock'];

/** "Mod+Shift+P": Mod is Cmd on a Mac and Ctrl elsewhere. Null while only modifiers are held. */
export function comboOf(e: KeyboardEvent | React.KeyboardEvent): string | null {
  if (MODIFIER_KEYS.includes(e.key)) return null;
  const parts: string[] = [];
  if (isMac ? e.metaKey : e.ctrlKey) parts.push('Mod');
  if (isMac ? e.ctrlKey : e.metaKey) parts.push(isMac ? 'Ctrl' : 'Meta');
  if (e.altKey) parts.push('Alt');
  if (e.shiftKey) parts.push('Shift');
  parts.push(e.key === ' ' ? 'Space' : e.key.length === 1 ? e.key.toUpperCase() : e.key);
  return parts.join('+');
}

const NAMES: Record<string, string> = { Escape: 'Esc', Delete: 'Del', ArrowUp: '↑', ArrowDown: '↓', ArrowLeft: '←', ArrowRight: '→' };

/** How a combination is shown. */
export function comboLabel(combo: string): string {
  if (!combo) return '';
  return combo.split('+').map((p) => {
    if (p === 'Mod') return isMac ? '⌘' : 'Ctrl';
    if (p === 'Shift') return isMac ? '⇧' : 'Shift';
    if (p === 'Alt') return isMac ? '⌥' : 'Alt';
    if (p === 'Ctrl') return '⌃';
    return NAMES[p] ?? p;
  }).join(isMac ? '' : '+');
}
