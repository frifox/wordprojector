import { useState } from 'react';
import { t } from '../i18n';
import type { Settings } from '../lib/types';
import { HOTKEYS, HOTKEY_GROUPS, hotkeyMap, comboOf, comboLabel, isReserved } from '../lib/hotkeys';

/** Settings > Hotkeys: the keys that work anywhere in the program, and the chance to choose others. */
export default function HotkeysPage({ s, setS }: { s: Settings; setS: (f: (x: Settings) => Settings) => void }) {
  const [rec, setRec] = useState<string | null>(null); // the action waiting for a key
  const [note, setNote] = useState('');
  const keys = hotkeyMap(s.app.hotkeys);
  const set = (next: Record<string, string>) => setS((x) => ({ ...x, app: { ...x.app, hotkeys: next } }));
  // the chosen keys are kept only where they differ from the defaults
  const store = (m: Record<string, string>) => set(Object.fromEntries(HOTKEYS.filter((h) => m[h.id] !== h.def).map((h) => [h.id, m[h.id]])));
  const assign = (id: string, combo: string) => {
    const m = { ...keys };
    // a key does one thing: the action that had it is left without
    for (const h of HOTKEYS) if (combo && m[h.id] === combo) m[h.id] = '';
    m[id] = combo;
    store(m);
  };
  const changed = HOTKEYS.some((h) => keys[h.id] !== h.def);
  return (
    <div className="col" style={{ gap: 6 }}>
      <div className="muted">{t('Click a key, then press the new combination.')}</div>
      {HOTKEY_GROUPS.map((g) => (
      <fieldset key={g.title} className="group"><legend>{t(g.title)}</legend>
        {g.items.map((h) => {
          const mine = keys[h.id];
          return (
            <div key={h.id} className="row" style={{ alignItems: 'center', gap: 8, padding: '4px 2px', borderBottom: '1px solid var(--border-light)' }}>
              <span className="grow">{t(h.label)}</span>
              <button
                style={{ minWidth: 130 }}
                className={rec === h.id ? 'primary' : ''}
                data-esc-own={rec === h.id ? '' : undefined}
                onClick={() => setRec(rec === h.id ? null : h.id)}
                onBlur={() => setRec((r) => (r === h.id ? null : r))}
                onKeyDown={(e) => {
                  if (rec !== h.id) return;
                  const combo = comboOf(e);
                  e.preventDefault();
                  e.stopPropagation();
                  if (!combo) return;
                  if (isReserved(combo)) { setNote(t('{key} is used by the program and cannot be assigned.', { key: comboLabel(combo) })); setRec(null); return; }
                  setNote('');
                  assign(h.id, combo);
                  setRec(null);
                }}
              >
                {rec === h.id ? t('Press a key…') : mine ? comboLabel(mine) : <span className="muted">{t('Not set')}</span>}
              </button>
              <button disabled={!mine} onClick={() => assign(h.id, '')} title={t('Clear')}>✕</button>
              <button disabled={mine === h.def} onClick={() => assign(h.id, h.def)} title={t('Reset')}>↺</button>
            </div>
          );
        })}
      </fieldset>
      ))}
      {note && <div className="muted">{note}</div>}
      <div className="row">
        <button disabled={!changed} onClick={() => set({})}>{t('Reset All')}</button>
      </div>
    </div>
  );
}
