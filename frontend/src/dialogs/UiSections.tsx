import { t } from '../i18n';
import type { Settings } from '../lib/types';

const UI_COLORS = {
  light: { bg: '#ececec', panel: '#f5f5f5', base: '#ffffff', line: '#c8c8c8', sel: '#cfe0f7', text: '#1c1c1c' },
  dark: { bg: '#2b2b2b', panel: '#333333', base: '#1e1e1e', line: '#555555', sel: '#2d4b73', text: '#e4e4e4' },
};

/** A tiny picture of the operator window in one appearance. "system" shows light and dark side by side. */
function UiThumb({ mode }: { mode: 'system' | 'light' | 'dark' }) {
  const half = (c: typeof UI_COLORS.light) => (
    <div style={{ width: '100%', height: '100%', background: c.bg, display: 'flex', flexDirection: 'column', gap: 2, padding: 3, boxSizing: 'border-box' }}>
      <div style={{ height: 6, background: c.panel, border: `1px solid ${c.line}`, borderRadius: 1 }} />
      <div style={{ flex: 1, display: 'flex', gap: 2 }}>
        <div style={{ width: '30%', background: c.base, border: `1px solid ${c.line}`, padding: 2, boxSizing: 'border-box', display: 'flex', flexDirection: 'column', gap: 2 }}>
          <div style={{ height: 4, background: c.sel }} /><div style={{ height: 3, background: c.line }} /><div style={{ height: 3, background: c.line }} />
        </div>
        <div style={{ flex: 1, background: c.base, border: `1px solid ${c.line}`, padding: 2, boxSizing: 'border-box', display: 'flex', flexDirection: 'column', gap: 2 }}>
          <div style={{ height: 3, background: c.text, opacity: 0.7, width: '80%' }} /><div style={{ height: 3, background: c.line }} /><div style={{ height: 3, background: c.line, width: '60%' }} />
        </div>
      </div>
    </div>
  );
  return (
    <div className="ui-thumb-pic">
      {mode === 'system' ? (
        <>
          <div style={{ position: 'absolute', inset: 0 }}>{half(UI_COLORS.light)}</div>
          <div style={{ position: 'absolute', inset: 0, clipPath: 'polygon(100% 0, 100% 100%, 0 100%)' }}>{half(UI_COLORS.dark)}</div>
        </>
      ) : half(UI_COLORS[mode])}
    </div>
  );
}

/** Settings > General: the look of the operator window. */
export function UserInterfaceSection({ s, setS, applyNow }: { s: Settings; setS: (f: (x: Settings) => Settings) => void; applyNow: (next: Settings) => Promise<void> }) {
  const setApp = (p: Partial<Settings['app']>) => setS((x) => ({ ...x, app: { ...x.app, ...p } }));
  return (
    <fieldset className="group">
        <legend>{t('Interface Theme')}</legend>
        <div className="row" style={{ gap: 12, alignItems: 'flex-start' }}>
          {(['system', 'light', 'dark'] as const).map((mode) => (
            <button key={mode} className={`ui-thumb ${(s.app.uiTheme || 'system') === mode ? 'on' : ''}`} onClick={() => setApp({ uiTheme: mode })} onDoubleClick={() => applyNow({ ...s, app: { ...s.app, uiTheme: mode } })} aria-pressed={(s.app.uiTheme || 'system') === mode}>
              <UiThumb mode={mode} />
              <span>{mode === 'system' ? t('System') : mode === 'light' ? t('Light') : t('Dark')}</span>
            </button>
          ))}
        </div>
    </fieldset>
  );
}

/** Presentation > General: how the displays change between slides. */
export function TransitionSection({ s, setS }: { s: Settings; setS: (f: (x: Settings) => Settings) => void }) {
  const tr = s.app.transition ?? { cut: false, fadeMillis: 500 };
  const setTr = (p: Partial<{ cut: boolean; fadeMillis: number }>) => setS((x) => ({ ...x, app: { ...x.app, transition: { ...tr, ...p } } }));
  return (
    <fieldset className="group">
        <legend>{t('Slide Transition')}</legend>
        <div className="row" style={{ gap: 16 }}>
          <label className="chk"><input type="radio" name="transition" checked={tr.cut} onChange={() => setTr({ cut: true })} />{t('Cut')}</label>
          <label className="chk"><input type="radio" name="transition" checked={!tr.cut} onChange={() => setTr({ cut: false })} />{t('Fade')}</label>
        </div>
        <div className="row" style={{ marginTop: 8, gap: 8 }}>
          <span style={{ opacity: tr.cut ? 0.45 : 1 }}>{t('Fade time:')}</span>
          <input type="range" min={100} max={3000} step={100} value={tr.fadeMillis} disabled={tr.cut} onChange={(e) => setTr({ fadeMillis: Number(e.target.value) })} />
          <span style={{ minWidth: 48, opacity: tr.cut ? 0.45 : 1 }}>{(tr.fadeMillis / 1000).toFixed(1)} s</span>
        </div>
    </fieldset>
  );
}
