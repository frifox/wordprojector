import { useEffect, useRef, useState } from 'react';
import { alertDialog } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { RenderSlide, Settings, ThemeInfo } from '../lib/types';
import ScaledSlide from '../projector/ScaledSlide';
import NameInfoDialog from './NameInfoDialog';

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

const THUMB_W = 160;
const THUMB_H = 90;

/** A scaled down sample slide in the look of a projection theme. */
function Thumb({ id, version }: { id: number; version: string }) {
  const [slide, setSlide] = useState<RenderSlide | null>(null);
  useEffect(() => {
    let live = true;
    api.themePreview(id).then((s) => live && setSlide(s)).catch(() => {});
    return () => { live = false; };
  }, [id, version]);
  return (
    <div style={{ width: THUMB_W, height: THUMB_H, position: 'relative', overflow: 'hidden', background: '#000', border: '1px solid var(--border)', borderRadius: 3, flex: 'none' }}>
      {slide && (
        <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
          <ScaledSlide slide={slide} width={THUMB_W} height={THUMB_H} />
        </div>
      )}
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

/** Settings > Presentation: the presentation theme (list with previews) and the transition between slides. */
export default function PresentationPage({ s, themes, currentId, changeTheme, onOpenEditor, applyNow }: {
  s: Settings; themes: ThemeInfo[]; currentId: number; changeTheme: (id: number) => void;
  /** opens the full window theme editor */
  onOpenEditor: () => void;
  /** saves the given settings right away (double clicking a thumbnail) */
  applyNow: (next: Settings) => Promise<void>;
}) {
  const { reloadSettings, dataVersion } = useStore();
  const [picked, setPicked] = useState(currentId);
  const [edit, setEdit] = useState<number | null>(null); // the id of the theme being renamed
  useEffect(() => setPicked(currentId), [currentId]); // Save As, in the bar below, makes a new theme the current one
  const listRef = useRef<HTMLDivElement>(null);
  const choose = (id: number) => {
    setPicked(id);
    changeTheme(id); // the projection theme in use (and the one the Passive, Bible, Songs pages edit)
  };

  return (
    <div className="col" style={{ gap: 8 }}>
      <fieldset className="group">
        <legend>{t('Presentation Theme')}</legend>
        <div ref={listRef} className="row" tabIndex={0} style={{ gap: 12, alignItems: 'flex-start', flexWrap: 'wrap', outline: 'none' }}
          onKeyDown={(e) => {
            const at = themes.findIndex((x) => x.id === picked);
            if ((e.key === 'ArrowRight' || e.key === 'ArrowDown') && at < themes.length - 1) { choose(themes[at + 1].id); e.preventDefault(); }
            else if ((e.key === 'ArrowLeft' || e.key === 'ArrowUp') && at > 0) { choose(themes[at - 1].id); e.preventDefault(); }
          }}>
          {themes.map((th) => (
            <div key={th.id} className="bg-tile" style={{ position: 'relative' }}>
              <button className={`ui-thumb ${th.id === picked ? 'on' : ''}`} onClick={() => choose(th.id)} onDoubleClick={() => { choose(th.id); applyNow({ ...s, general: { ...s.general, currentThemeId: th.id } }); }} aria-pressed={th.id === picked}>
                <Thumb id={th.id} version={`${dataVersion.themes}-${th.name}`} />
                <span>{th.name}</span>
              </button>
              {!th.builtin && (
                <div className="bg-actions">
                  <button title={t('Rename')} aria-label={t('Rename')} onClick={() => setEdit(th.id)}>
                    <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4" /><path d="M13.5 6.5l4 4" /></svg>
                  </button>
                </div>
              )}
            </div>
          ))}
        </div>
        <div className="row" style={{ marginTop: 6, flexWrap: 'wrap' }}>
          <span className="grow" />
          <button onClick={onOpenEditor}>{t('Theme Editor')}</button>
        </div>
      </fieldset>
      {edit !== null && (
        <NameInfoDialog title={t('Rename Theme')} nameLabel={t('Theme Name:')} name={themes.find((x) => x.id === edit)?.name}
          onClose={() => setEdit(null)}
          onSave={async (n) => {
            try {
              await api.renameTheme(edit, n);
              await reloadSettings();
              setEdit(null);
            } catch (e) { alertDialog(t('Error'), String((e as Error).message || e)); }
          }} />
      )}
    </div>
  );
}
