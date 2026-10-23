// Minimal i18n: t(key) returns the translation of an English source string.
// Dictionaries are keyed by the English string, falling back to it.
import ru from './ru.json';
import de from './de.json';
import cs from './cs.json';
import ua from './ua.json';
import hy from './hy.json';

type Dict = Record<string, string>;
const dicts: Record<string, Dict> = { ru, de, cs, ua, hy };

export const languages: { code: string; name: string; flag: string }[] = [
  { code: 'en', name: 'English', flag: 'flag_uk.png' },
  { code: 'ru', name: 'Русский', flag: 'flag_ru.png' },
  { code: 'de', name: 'Deutsch', flag: 'flag_de.png' },
  { code: 'cs', name: 'Čeština', flag: 'flag_cs.png' },
  { code: 'ua', name: 'Українська', flag: 'flag_ua.png' },
  { code: 'hy', name: 'Հայերեն', flag: 'flag_hy.png' },
];

let current = 'en';
const listeners = new Set<() => void>();

export function setLanguage(code: string) {
  current = dicts[code] ? code : 'en';
  listeners.forEach((l) => l());
}

export function getLanguage() {
  return current;
}

export function onLanguageChange(l: () => void) {
  listeners.add(l);
  return () => {
    listeners.delete(l);
  };
}

export function t(s: string, vars?: Record<string, string | number>): string {
  const d = dicts[current];
  let out = (d && d[s]) || s;
  if (vars) {
    for (const [k, v] of Object.entries(vars)) out = out.split(`{${k}}`).join(String(v));
  }
  return out;
}
