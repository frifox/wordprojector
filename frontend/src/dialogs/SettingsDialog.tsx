import { useEffect, useRef, useState } from 'react';
import Modal, { alertDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { BackgroundSet, BibleSettings, PassiveSettings, Settings, SongSettings, TextSettings, Screen, ThemeImage } from '../lib/types';
import { FontPicker, FontStyleButtons, ColorField, Select, AlignGroup, IconChoice, POSITION_ICONS } from '../components/Pickers';
import { pickFiles, imageFilter } from '../lib/files';
import PresentationPage, { UserInterfaceSection } from './PresentationPage';
import ThemeEditor from './ThemeEditor';
import UpdatesPage from './UpdatesPage';
import ScreenMap from '../components/ScreenMap';
import { CountdownDisc } from '../projector/SlideView';

type Page = 'general' | 'presentation' | 'passive' | 'remote' | 'updates';

export default function SettingsDialog({ onClose, initialPage }: { onClose: () => void; initialPage?: Page }) {
  const store = useStore();
  const [page, setPage] = useState<Page>(initialPage ?? 'general');
  const [s, setS] = useState<Settings>(() => JSON.parse(JSON.stringify(store.settings)));
  const [screens, setScreens] = useState<Screen[]>(store.boot.screens || []);
  // the background of the passive screen belongs to the program, not to a theme: it has its own settings
  const [bg, setBg] = useState<BackgroundSet | null>(null);
  const [bgDirty, setBgDirty] = useState(false);
  useEffect(() => { api.backgroundTheme().then(setBg).catch(() => {}); }, []);
  const baseline = useRef(JSON.stringify(store.settings));
  const dirty = bgDirty || JSON.stringify(s) !== baseline.current;
  const requestClose = useDiscardGuard(dirty, onClose, () => ok());
  // screens can be plugged in or removed while the dialog is open: reload on the server's notice, and now and then
  // as a fallback (browser mode has no notice)
  useEffect(() => {
    const load = () => api.screens().then(setScreens).catch(() => {});
    load();
    const id = window.setInterval(load, 3000);
    return () => window.clearInterval(id);
  }, [store.dataVersion.screens]);

  const setGeneral = (p: Partial<Settings['general']>) => setS((x) => ({ ...x, general: { ...x.general, ...p } }));
  const updBg = (fn: (t: BackgroundSet) => void) => {
    setBg((x) => {
      if (!x) return x;
      const n: BackgroundSet = JSON.parse(JSON.stringify(x));
      fn(n);
      return n;
    });
    setBgDirty(true);
  };

  // `next` lets a caller apply a change that is not in state yet (double clicking a theme thumbnail)
  const apply = async (next: Settings = s) => {
    try {
      if (bgDirty && bg) await api.saveBackground(bg);
      await api.saveSettings(next);
      setBgDirty(false);
      baseline.current = JSON.stringify(next);
      await store.reloadSettings();
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
  };
  const ok = async () => {
    await apply();
    onClose();
  };
  const changeTheme = (id: number) => setGeneral({ currentThemeId: id });

  const multi = screens.length !== 1;
  const activeDisplays = [1, ...(multi && s.general.displayScreen2 >= 0 ? [2] : []), ...(multi && s.general.displayScreen3 >= 0 ? [3] : []), ...(multi && s.general.displayScreen4 >= 0 ? [4] : [])];

  const pages: { id: Page; label: string }[] = [
    { id: 'general', label: t('General') }, { id: 'passive', label: t('Background') }, { id: 'presentation', label: t('Presentation') },
    { id: 'remote', label: t('Remote') },
    { id: 'updates', label: t('Updates') },
  ];
  const section: Page = page;
  const [editorOpen, setEditorOpen] = useState(false); // the full window theme editor
  const headers: Record<Page, string> = { general: t('General Word Projector Settings'), presentation: t('Presentation Settings'), passive: t('Background Settings'), remote: t('Remote Access Settings'), updates: t('Updates') };

  return (
    <Modal title={t('Word Projector - Settings')} onClose={requestClose} width={900} height="90vh"
      buttons={<><button onClick={requestClose}>{t('Cancel')}</button><button onClick={() => apply()} disabled={!dirty}>{t('Apply')}</button><button className="primary" onClick={ok}>{t('OK')}</button></>}>
      <div className="row grow" style={{ alignItems: 'stretch', minHeight: 0 }}>
        <div className="list" style={{ width: 150, flex: 'none' }}>
          {pages.map((p) => <div key={p.id} className={`item ${page === p.id ? 'sel' : ''}`} style={{ padding: '6px 10px' }} onClick={() => setPage(p.id)}>{p.label}</div>)}
        </div>
        <div className="col grow" style={{ minHeight: 0, minWidth: 0, gap: 6 }}>
        <div className="col grow" style={{ overflow: 'auto', minHeight: 0, paddingRight: 4 }}>
          <div style={{ fontWeight: 600, fontSize: 14, borderBottom: '1px solid var(--border-light)', paddingBottom: 4 }}>{headers[page]}</div>
          {section === 'passive' && <div className="muted">{t('These settings are displayed when nothing is to be projected.')}</div>}
          {page === 'general' && <GeneralPage s={s} setS={setS} setGeneral={setGeneral} screens={screens} applyNow={(next) => { setS(next); return apply(next); }} />}
          {page === 'presentation' && <PresentationPage s={s} onOpenEditor={() => setEditorOpen(true)} themes={store.themes} currentId={s.general.currentThemeId} changeTheme={changeTheme} applyNow={(next) => { setS(next); return apply(next); }} />}
          {page === 'passive' && bg && activeDisplays.map((d) => (
            <DisplaySection key={d} d={d} useDisp1={d > 1 ? bg.passive[d - 1].useDisp1 : false} setUseDisp1={(v) => updBg((tm) => { tm.passive[d - 1].useDisp1 = v; })}>
              <BackgroundRow themeId={bg.id} kind="passive" d={d} v={bg.passive[d - 1]} onChange={(p) => updBg((tm) => { tm.passive[d - 1] = { ...tm.passive[d - 1], ...p }; })} />
            </DisplaySection>
          ))}
          {page === 'remote' && <RemotePage s={s} setS={setS} />}
          {page === 'updates' && <UpdatesPage s={s} setS={setS} />}
        </div>
        {editorOpen && (
          <ThemeEditor s={s} setS={setS} currentId={s.general.currentThemeId}
            onUse={(id) => { const next = { ...s, general: { ...s.general, currentThemeId: id } }; setS(next); apply(next); }}
            onClose={() => { setEditorOpen(false); store.reloadSettings(); }} />
        )}
        </div>
      </div>
    </Modal>
  );
}

export function DisplaySection({ d, useDisp1, setUseDisp1, children }: { d: number; useDisp1: boolean; setUseDisp1: (v: boolean) => void; children: React.ReactNode }) {
  if (d === 1) return <>{children}</>;
  const names = ['', '', t('Secondary'), t('Tertiary'), t('Quaternary')];
  return (
    <fieldset className="group">
      <legend><label className="chk"><input type="checkbox" checked={!useDisp1} onChange={(e) => setUseDisp1(!e.target.checked)} />{t('Use Separate {name} Display Screen Settings', { name: names[d] })}</label></legend>
      {!useDisp1 && children}
    </fieldset>
  );
}

function GeneralPage({ s, setS, setGeneral, screens, applyNow }: { s: Settings; setS: (f: (x: Settings) => Settings) => void; setGeneral: (p: Partial<Settings['general']>) => void; screens: Screen[]; applyNow: (next: Settings) => Promise<void> }) {
  const g = s.general;
  const { boot: bootInfo } = useStore();
  const [testing, setTesting] = useState(false);
  const runTest = async () => {
    setTesting(true);
    try {
      await api.testDisplays([g.displayScreen, g.displayScreen2, g.displayScreen3, g.displayScreen4, ...(g.displayScreensExtra || [])]);
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
    window.setTimeout(() => setTesting(false), 3000);
  };
  const multi = screens.length !== 1;
  const dim = s.app.hideDisplayControls ? { opacity: 0.45 } : undefined; // labels fade with the controls they belong to
  // the server asks the operating system for the screens; if that fails, fall back to the screen this page is on
  const browserScreens: Screen[] = [{ index: 0, name: '', x: 0, y: 0, width: window.screen.width, height: window.screen.height, isPrimary: true }];
  const mapScreens = screens.length ? screens : browserScreens;
  return (
    <div className="col" style={{ gap: 4 }}>
      <UserInterfaceSection s={s} setS={setS} applyNow={applyNow} />
      <fieldset className="group">
        <legend>{t('Display Screens')}</legend>
        {!bootInfo.desktop && <div className="muted" style={{ marginBottom: 6 }}>{t('Running without native windows: open /projector/1, /projector/2 … in a browser on each display screen.')}</div>}
        {screens.length === 1 && <div className="muted" style={{ marginBottom: 6 }}>{t('Only one screen detected. The display window opens on the main screen while something is shown.')}</div>}
        <ScreenMap screens={mapScreens} slots={[g.displayScreen, g.displayScreen2, g.displayScreen3, g.displayScreen4, ...(g.displayScreensExtra || [])]}
          onChange={(sl) => setGeneral({ displayScreen: Math.max(0, sl[0] ?? 0), displayScreen2: sl[1] ?? -1, displayScreen3: sl[2] ?? -1, displayScreen4: sl[3] ?? -1, displayScreensExtra: sl.slice(4) })} />
        {!bootInfo.desktop && (
          <div className="muted" style={{ marginTop: 6, lineHeight: 1.6 }}>
            {t('This is the browser version: it cannot open windows on other screens by itself. Open a display page in a browser window on each screen:')}{' '}
            {[g.displayScreen, g.displayScreen2, g.displayScreen3, g.displayScreen4, ...(g.displayScreensExtra || [])].filter((x, i) => i === 0 || x >= 0).map((_, i) => (
              <a key={i} href={`/projector/${i + 1}`} target="_blank" rel="noreferrer" style={{ marginRight: 12 }}>{t('Display {n}', { n: i + 1 })}</a>
            ))}
          </div>
        )}
        <div className="row" style={{ marginTop: 6 }}>
          <label className="chk"><input type="checkbox" checked={g.displayOnStartUp} onChange={(e) => setGeneral({ displayOnStartUp: e.target.checked })} />{t('Start presenting on startup')}</label>
          <span className="grow" />
          <button aria-busy={testing} onClick={() => { if (!testing) runTest(); }} title={t('Show a test pattern on the selected screens for 3 seconds')}>
            {testing && <CountdownDisc size={14} millis={3000} />}
            {t('Test')}
          </button>
        </div>
      </fieldset>
      <fieldset className="group" disabled={multi}>
        <legend>{t('Display Controls')}</legend>
        <label className="chk" style={{ marginBottom: 6 }}><input type="checkbox" checked={!s.app.hideDisplayControls} onChange={(e) => setS((x) => ({ ...x, app: { ...x.app, hideDisplayControls: !e.target.checked } }))} />{t('Show display controls (when in single screen mode)')}</label>
        <fieldset disabled={!!s.app.hideDisplayControls} style={{ border: 0, padding: 0, margin: 0, minWidth: 0 }}>
        <div className="form-grid">
          <span style={dim}>{t('Alignment:')}</span>
          <div className="row" style={{ gap: 8 }}>
            <AlignGroup axis="v" h={g.displayControls.alignH} value={g.displayControls.alignV} onChange={(v) => setGeneral({ displayControls: { ...g.displayControls, alignV: v } })} />
            <AlignGroup axis="h" value={g.displayControls.alignH} onChange={(v) => setGeneral({ displayControls: { ...g.displayControls, alignH: v } })} />
            <span style={{ marginLeft: 8, ...dim }}>{t('Icon size:')}</span>
            <Select value={g.displayControls.buttonSize} options={['16', '24', '32', '48', '64', '96']} onChange={(v) => setGeneral({ displayControls: { ...g.displayControls, buttonSize: v } })} />
          </div>
          <span style={dim}>{t('Opacity:')}</span><div className="row"><input type="range" min={0} max={100} value={Math.round(g.displayControls.opacity * 100)} onChange={(e) => setGeneral({ displayControls: { ...g.displayControls, opacity: Number(e.target.value) / 100 } })} /><span style={dim}>{Math.round(g.displayControls.opacity * 100)}%</span></div>
        </div>
        </fieldset>
      </fieldset>
    </div>
  );
}

/** One image in the collection. While the picture is still being added or loaded it shows a spinner instead. */
function BgTile({ src, name, selected, aspect, onChoose, onRemove, onRename }: { src: string | null; name: string; selected: boolean; aspect: number; onChoose?: () => void; onRemove?: () => void; onRename?: (name: string) => void }) {
  const [loaded, setLoaded] = useState(false);
  const [editing, setEditing] = useState<string | null>(null); // the new name while it is being typed
  const finishRename = (save: boolean) => {
    const v = (editing ?? '').trim();
    setEditing(null);
    if (save && v && v !== name) onRename?.(v);
  };
  const ready = !!src && loaded;
  return (
    <div style={{ position: 'relative' }} className="bg-tile">
      <button className={`ui-thumb ${selected ? 'on' : ''}`} onClick={onChoose} disabled={!ready} aria-pressed={selected} aria-busy={!ready} title={name}>
        <div style={{ width: 120, aspectRatio: String(aspect), background: '#000', border: '1px solid var(--border)', borderRadius: 3, overflow: 'hidden', position: 'relative' }}>
          {src && <img src={src} alt="" draggable={false} onLoad={() => setLoaded(true)} onError={() => setLoaded(true)}
            ref={(el) => { if (el && el.complete && el.naturalWidth > 0 && !loaded) setLoaded(true); }}
            style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', opacity: ready ? 1 : 0 }} />}
          {!ready && <div className="bg-spinner-wrap"><div className="bg-spinner" /></div>}
        </div>
        {editing === null && <span style={{ maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12 }}>{name}</span>}
      </button>
      {editing !== null && (
        <input data-esc-own autoFocus value={editing} onChange={(e) => setEditing(e.target.value)} onFocus={(e) => e.currentTarget.select()} onBlur={() => finishRename(true)}
          onKeyDown={(e) => { if (e.key === 'Enter') finishRename(true); else if (e.key === 'Escape') { e.stopPropagation(); finishRename(false); } }}
          style={{ position: 'absolute', left: 6, right: 6, bottom: 6, width: 'auto', fontSize: 12, padding: '1px 4px' }} />
      )}
      {ready && editing === null && (onRename || onRemove) && (
        <div className="bg-actions">
          {onRename && (
            <button title={t('Rename')} aria-label={t('Rename')} onClick={() => setEditing(name)}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4" /><path d="M13.5 6.5l4 4" /></svg>
            </button>
          )}
          {onRemove && (
            <button title={t('Remove')} aria-label={t('Remove')} onClick={onRemove}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/** Little pictures of the three ways an image can fill the screen (the screen is the outline). */
function ScaleIcon({ mode }: { mode: number }) {
  const img = 'var(--accent)';
  return (
    <svg viewBox="0 0 56 44" width={56} height={44} aria-hidden="true" style={{ display: 'block', color: 'var(--text)' }}>
      <defs><clipPath id={`screen-${mode}`}><rect x="4" y="9" width="48" height="26" /></clipPath></defs>
      {mode === 1 && (
        <>
          {/* Fill: the image is bigger than the screen, so its top and bottom are cut off */}
          <rect x="4" y="1" width="48" height="42" fill="none" stroke="currentColor" strokeWidth="1" strokeDasharray="2 2" opacity="0.6" />
          <rect x="4" y="1" width="48" height="42" fill={img} clipPath={`url(#screen-${mode})`} />
        </>
      )}
      {mode === 2 && (
        <>
          {/* Fit: the whole image, with bars where the proportions differ */}
          <rect x="4" y="9" width="48" height="26" fill="#000" />
          <rect x="15" y="9" width="26" height="26" fill={img} />
        </>
      )}
      {mode === 0 && (
        <>
          {/* Scale: stretched edge to edge in both directions */}
          <rect x="4" y="9" width="48" height="26" fill={img} />
          <g stroke="#fff" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" fill="none">
            <path d="M12 22h32M12 22l4 -3M12 22l4 3M44 22l-4 -3M44 22l-4 3" />
            <path d="M28 14v16M28 14l-3 4M28 14l3 4M28 30l-3 -4M28 30l3 -4" />
          </g>
        </>
      )}
      <rect x="4" y="9" width="48" height="26" fill="none" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

function BackgroundRow({ themeId, kind, d, v, onChange }: { themeId: number; kind: string; d: number; v: PassiveSettings; onChange: (p: Partial<PassiveSettings>) => void }) {
  const { boot } = useStore();
  // the images this theme keeps for this slide kind and display; the selected one is what the slides show
  const [images, setImages] = useState<ThemeImage[]>([]);
  const [pending, setPending] = useState<string[]>([]);
  const selected = images.find((i) => i.selected);
  const apply = (list: ThemeImage[]) => {
    setImages(list);
    const sel = list.find((i) => i.selected);
    onChange({ backgroundName: sel?.name || '', hasBackground: !!sel, useBackground: !!sel });
  };
  useEffect(() => {
    api.themeImages(themeId, kind, d).then(setImages).catch(() => setImages([]));
  }, [themeId, kind, d]);
  // pixel size of the selected image, for the size indicator
  useEffect(() => {
    if (!selected) { setNatural(null); return; }
    const im = new Image();
    im.onload = () => setNatural({ w: im.naturalWidth, h: im.naturalHeight });
    im.src = `/api/theme-images/${selected.id}`;
  }, [selected?.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const add = async () => {
    const paths = await pickFiles(boot.desktop, t('Select images for the wallpaper'), [imageFilter], true);
    if (!paths.length) return;
    setPending(paths.map((p) => p.split(/[\\/]/).pop() || p));
    try { apply(await api.addThemeImages(themeId, kind, d, paths)); } catch (e) { alertDialog(t('Error'), String((e as Error).message || e)); } finally { setPending([]); }
  };
  const choose = async (img: ThemeImage) => { if (!img.selected) apply(await api.selectThemeImage(themeId, kind, d, img.id)); };
  const renameImg = async (img: ThemeImage, name: string) => {
    try { apply(await api.renameThemeImage(themeId, kind, d, img.id, name)); } catch (e) { alertDialog(t('Error'), String((e as Error).message || e)); }
  };
  const removeImg = async (img: ThemeImage) => { apply(await api.removeThemeImage(themeId, kind, d, img.id)); };
  const passive = kind === 'passive';
  const fit = passive ? ((v as PassiveSettings).backgroundFit ?? 0) : 0; // 0 scale, 1 fill, 2 fit (the tiles show Fill, Fit, Scale)
  // size of the picture and of the screen it is shown on, for the "% of original" indicator
  const [natural, setNatural] = useState<{ w: number; h: number } | null>(null);
  const { live } = useStore();
  const disp = live.displays.find((x) => x.disp === d) || live.displays[0];
  const screenW = disp?.width || 1920;
  const screenH = disp?.height || 1080;
  let info: { image: string; screen: string; shown: string } | null = null;
  if (passive && natural && selected) {
    const sx = screenW / natural.w;
    const sy = screenH / natural.h;
    const pct = (x: number) => `${Math.round(x * 100)}%`;
    const shown = fit === 1 ? pct(Math.max(sx, sy)) : fit === 2 ? pct(Math.min(sx, sy)) : Math.abs(sx - sy) < 0.005 ? pct(sx) : `${pct(sx)} × ${pct(sy)}`;
    info = { image: `${natural.w} × ${natural.h} px`, screen: `${screenW} × ${screenH} px`, shown };
  }
  return (
    <fieldset className="group">
      <legend><label className="chk"><input type="checkbox" checked={v.useBackground} onChange={(e) => onChange({ useBackground: e.target.checked })} />{t('Use Background Image')}</label></legend>
      {/* the collection: its own framed box with a header bar and a scrolling area of thumbnails */}
      <div style={{ border: '1px solid var(--border)', borderRadius: 4, background: 'var(--base)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="row" style={{ padding: '4px 8px', background: 'var(--panel)', borderBottom: '1px solid var(--border)' }}>
          <b style={{ fontSize: 12 }}>{t('Images')}</b>
          <span className="muted" style={{ fontSize: 12 }}>{images.length}</span>
          <span className="grow" />
          <button className="small" onClick={add} title={t('Add images...')}>{t('Add...')}</button>
        </div>
        <div className="row" style={{ gap: 10, alignItems: 'flex-start', flexWrap: 'wrap', padding: 8, minHeight: 90, maxHeight: 260, overflow: 'auto' }}>
          {images.map((img) => (
            <BgTile key={img.id} src={`/api/theme-images/${img.id}`} name={img.name} selected={img.selected} aspect={screenW / screenH} onChoose={() => choose(img)} onRemove={() => removeImg(img)} onRename={(n) => renameImg(img, n)} />
          ))}
          {/* files being added: placeholders with a spinner until the server has them ready */}
          {pending.map((name, i) => <BgTile key={`pending-${i}-${name}`} src={null} name={name} selected={false} aspect={screenW / screenH} />)}
          {images.length === 0 && pending.length === 0 && <div className="muted" style={{ alignSelf: 'center', width: '100%', textAlign: 'center' }}>{t('No images yet. Use Add... to choose some.')}</div>}
        </div>
      </div>
      {passive && (
        <div className="col" style={{ marginTop: 6, gap: 4 }}>
          <div className="row" style={{ gap: 10, alignItems: 'flex-start', flexWrap: 'wrap' }} role="radiogroup" aria-label={t('Image scaling:')}>
            {([[1, t('Fill')], [2, t('Fit')], [0, t('Scale')]] as const).map(([value, label]) => (
              <button key={value} className={`ui-thumb ${fit === value ? 'on' : ''}`} disabled={!selected} role="radio" aria-checked={fit === value}
                onClick={() => onChange({ backgroundFit: value })}
                title={value === 1 ? t('Fills the whole screen; the edges of the image may be cut off.') : value === 2 ? t('Shows the whole image; black bars appear when its proportions differ from the screen.') : t('Stretches the image edge to edge and ignores its proportions.')}>
                <ScaleIcon mode={value} />
                <span>{label}</span>
              </button>
            ))}
          </div>
          {info && (
            <div className="form-grid" style={{ gridTemplateColumns: 'auto auto', justifyContent: 'start', columnGap: 12, rowGap: 2, fontSize: 12 }}>
              <span className="muted">{t('Image size:')}</span><span>{info.image}</span>
              <span className="muted">{t('Screen size:')}</span><span>{info.screen}</span>
              <span className="muted">{t('Shown at:')}</span><span style={{ fontWeight: 600 }}>{info.shown}</span>
            </div>
          )}
        </div>
      )}
    </fieldset>
  );
}

/** Horizontal scale of the characters, in percent of normal (an icon, the number and %). */
function ScaleField({ value, onChange }: { value?: number; onChange: (n: number) => void }) {
  return (
    <span className="row" style={{ gap: 4, marginLeft: 10 }} title={t('Horizontal scale: the width of the characters, in percent of normal. 50 makes them half as wide, 200 twice as wide.')}>
          <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="square" strokeLinejoin="miter" role="img" aria-label={t('Horizontal scale')}>
            <path d="M6 3.5h12M6 3.5v2.5M18 3.5v2.5M12 3.5v11.5M10 15h4" />
            <path d="M4 19.5h16M6.2 17.7l-2.2 1.8l2.2 1.8M17.8 17.7l2.2 1.8l-2.2 1.8" strokeWidth="1.2" strokeLinecap="butt" />
          </svg>
          <input type="number" min={20} max={300} value={value ?? 100} style={{ width: 52 }} aria-label={t('Horizontal scale')}
            onChange={(e) => onChange(Math.max(1, Math.min(300, Math.round(Number(e.target.value) || 100))))} />
          <span className="muted">%</span>
        </span>
  );
}

function TextProps({ title, v, onChange, onPreview }: { title: string; v: TextSettings; onChange: (p: Partial<TextSettings>) => void; onPreview?: (p: Partial<TextSettings> | null) => void }) {
  return (
    <fieldset className="group"><legend>{title}</legend>
      <div className="row"><FontPicker noStyle value={v.textFont} onChange={(f) => onChange({ textFont: f })} onPreview={onPreview && ((f) => onPreview(f && { textFont: f }))}color={
        <ScaleField value={v.scaleH} onChange={(n) => onChange({ scaleH: n })} />
      } /></div>
      <div className="row" style={{ marginTop: 6 }}><ColorField bare value={v.textColor} onChange={(c) => onChange({ textColor: c })} /><FontStyleButtons value={v.textFont} onChange={(f) => onChange({ textFont: f })} /><AlignGroup axis="h" value={v.alignH} onChange={(x) => onChange({ alignH: x })} /><AlignGroup axis="v" h={v.alignH} value={v.alignV} onChange={(x) => onChange({ alignV: x })} /></div>
    </fieldset>
  );
}

function ScreenUse({ use, pos, onChange }: { use: number; pos: number; onChange: (p: { screenUse?: number; screenPosition?: number }) => void }) {
  return (
    <fieldset className="group"><legend>{t('Amount Of Screen To Use')}</legend>
      <div className="row"><span>{t('Vertical Screen Use:')}</span><input type="number" min={10} max={100} value={use} style={{ width: 60 }} onChange={(e) => onChange({ screenUse: Math.max(10, Math.min(100, Number(e.target.value) || 100)) })} /><span>%</span>
        {/* the block fills the whole height at 100%, so there is nothing to position */}
        <IconChoice value={pos} disabled={use >= 100} onChange={(x) => onChange({ screenPosition: x })} items={[{ value: 0, label: t('Top of screen'), icon: POSITION_ICONS.screenTop }, { value: 2, label: t('Center of screen'), icon: POSITION_ICONS.screenCenter }, { value: 1, label: t('Bottom of screen'), icon: POSITION_ICONS.screenBottom }]} /></div>
    </fieldset>
  );
}

function AddBk({ add, rec, padding, opacity, onChange }: { add: boolean; rec: string; padding: number; opacity: number; onChange: (p: { addBkColorToText?: boolean; textRecBkColor?: string; bkPadding?: number; bkOpacity?: number }) => void }) {
  return (
    <fieldset className="group"><legend><label className="chk"><input type="checkbox" checked={add} onChange={(e) => onChange({ addBkColorToText: e.target.checked })} />{t('Add Background to Text')}</label></legend>
      <div className="row">
        <ColorField bare value={rec} onChange={(c) => onChange({ textRecBkColor: c })} />
        <span className="muted" style={{ marginLeft: 10 }}>{t('Padding:')}</span>
        <input type="number" min={0} max={500} value={padding} style={{ width: 64 }} title={t('Space above and below the text, inside the background color. It is relative to the screen height.')}
          onChange={(e) => onChange({ bkPadding: Math.max(0, Math.min(500, Math.round(Number(e.target.value) || 0))) })} />
        <span className="muted" style={{ marginLeft: 10 }}>{t('Opacity:')}</span>
        <input type="number" min={0} max={100} value={opacity} style={{ width: 64 }} title={t('How opaque the background color is, in percent.')}
          onChange={(e) => onChange({ bkOpacity: Math.max(0, Math.min(100, Math.round(Number(e.target.value) || 0))) })} />
        <span className="muted">%</span>
      </div>
    </fieldset>
  );
}

export function BiblePage({ v, onChange, onPreview }: { v: BibleSettings; onChange: (p: Partial<BibleSettings>) => void; onPreview?: (p: Partial<BibleSettings> | null) => void }) {
  return (
    <div className="col" style={{ gap: 2 }}>
      <TextProps title={t('Text')} v={v} onChange={onChange} onPreview={onPreview} />
      <fieldset className="group"><legend>{t('Caption')}</legend>
        <div className="row"><FontPicker noStyle value={v.captionFont} onChange={(f) => onChange({ captionFont: f })} color={<ScaleField value={v.captionScaleH} onChange={(n) => onChange({ captionScaleH: n })} />} onPreview={onPreview && ((f) => onPreview(f && { captionFont: f }))} /></div>
        <div className="row" style={{ marginTop: 6 }}>
          <ColorField bare value={v.captionColor} onChange={(c) => onChange({ captionColor: c })} /><FontStyleButtons value={v.captionFont} onChange={(f) => onChange({ captionFont: f })} /><AlignGroup axis="h" value={v.captionAlign} onChange={(x) => onChange({ captionAlign: x })} />
          <IconChoice value={v.captionPosition} onChange={(x) => onChange({ captionPosition: x })} items={[{ value: 0, label: t('Above Text'), icon: POSITION_ICONS.above }, { value: 1, label: t('Below Text'), icon: POSITION_ICONS.below }]} />
        </div>
      </fieldset>
      <AddBk add={v.addBkColorToText} rec={v.textRecBkColor} padding={v.bkPadding ?? 10} opacity={v.bkOpacity ?? 100} onChange={onChange} />
      <ScreenUse use={v.screenUse} pos={v.screenPosition} onChange={onChange} />
    </div>
  );
}

export function SongPage({ v, onChange, onPreview }: { v: SongSettings; onChange: (p: Partial<SongSettings>) => void; onPreview?: (p: Partial<SongSettings> | null) => void }) {
  return (
    <div className="col" style={{ gap: 2 }}>
      <TextProps title={t('Text')} v={v} onChange={onChange} onPreview={onPreview} />
      <fieldset className="group"><legend>{t('Caption')}</legend>
        <div className="row"><FontPicker noStyle value={v.infoFont} onChange={(f) => onChange({ infoFont: f })} color={<ScaleField value={v.infoScaleH} onChange={(n) => onChange({ infoScaleH: n })} />} onPreview={onPreview && ((f) => onPreview(f && { infoFont: f }))} /></div>
        <div className="row" style={{ marginTop: 6 }}><ColorField bare value={v.infoColor} onChange={(c) => onChange({ infoColor: c })} /><FontStyleButtons value={v.infoFont} onChange={(f) => onChange({ infoFont: f })} /><IconChoice value={v.infoAlign} onChange={(x) => onChange({ infoAlign: x })} items={[{ value: 0, label: t('Above Text'), icon: POSITION_ICONS.above }, { value: 1, label: t('Below Text'), icon: POSITION_ICONS.below }]} /></div>
        <div className="col" style={{ gap: 2, marginTop: 4, alignItems: 'flex-start' }}>
          <label className="chk"><input type="checkbox" checked={v.showStanzaTitle} onChange={(e) => onChange({ showStanzaTitle: e.target.checked })} />{t('Show Stanza Title')}</label>
          <label className="chk"><input type="checkbox" checked={v.showKey} onChange={(e) => onChange({ showKey: e.target.checked })} />{t('Show Song Key')}</label>
          <label className="chk"><input type="checkbox" checked={v.showNumber} onChange={(e) => onChange({ showNumber: e.target.checked })} />{t('Show Song Number')}</label>
        </div>
      </fieldset>
      <fieldset className="group"><legend><label className="chk"><input type="checkbox" checked={v.showEnding} onChange={(e) => onChange({ showEnding: e.target.checked })} />{t('Show Song Ending')}</label></legend>
        <div className="row"><FontPicker noStyle value={v.endingFont} onChange={(f) => onChange({ endingFont: f })} color={<ScaleField value={v.endingScaleH} onChange={(n) => onChange({ endingScaleH: n })} />} onPreview={onPreview && ((f) => onPreview(f && { endingFont: f }))} /></div>
        <div className="row" style={{ marginTop: 6 }}>
          <ColorField bare value={v.endingColor} onChange={(c) => onChange({ endingColor: c })} /><FontStyleButtons value={v.endingFont} onChange={(f) => onChange({ endingFont: f })} />
          <IconChoice value={v.endingPosition} onChange={(x) => onChange({ endingPosition: x })} items={[{ value: 0, label: t('Below Song Text'), icon: POSITION_ICONS.belowText }, { value: 1, label: t('Bottom of Screen'), icon: POSITION_ICONS.bottomOfScreen }]} />
        </div>
        <div className="row" style={{ marginTop: 4 }}>
          <span>{t('Type:')}</span><Select value={v.endingType} options={['*   *   *', '-   -   -', '°   °   °', '•   •   •', '●   ●   ●', '▪   ▪   ▪', '■   ■   ■']} onChange={(x) => onChange({ endingType: x })} />
        </div>
      </fieldset>
      <AddBk add={v.addBkColorToText} rec={v.textRecBkColor} padding={v.bkPadding ?? 10} opacity={v.bkOpacity ?? 100} onChange={onChange} />
      <ScreenUse use={v.screenUse} pos={v.screenPosition} onChange={onChange} />
    </div>
  );
}

/** Where the backend listens. The server binds once at start up, so changes apply after a restart. */
function RemotePage({ s, setS }: { s: Settings; setS: (f: (x: Settings) => Settings) => void }) {
  const remote = s.app.remote ?? { public: false, port: 8777 };
  const [info, setInfo] = useState<{ listen: string; urls: string[] } | null>(null);
  const [portText, setPortText] = useState(String(remote.port || 8777));
  useEffect(() => { api.remoteInfo().then(setInfo).catch(() => setInfo(null)); }, []);
  const set = (p: Partial<{ public: boolean; port: number }>) => setS((x) => ({ ...x, app: { ...x.app, remote: { ...remote, ...p } } }));
  const portNum = Number(portText);
  const portOK = Number.isInteger(portNum) && portNum >= 1 && portNum <= 65535;
  return (
    <div className="col" style={{ gap: 8 }}>
      <fieldset className="group">
        <legend>{t('Backend Address')}</legend>
        <div className="form-grid">
          <span>{t('Host:')}</span>
          <span>{remote.public ? '0.0.0.0' : '127.0.0.1'}</span>
          <span>{t('Port:')}</span>
          <div className="row" style={{ gap: 6 }}>
            <input className="no-spin" type="number" min={1} max={65535} style={{ width: 90 }} value={portText} aria-invalid={!portOK}
              onChange={(e) => { setPortText(e.target.value); const n = Number(e.target.value); if (Number.isInteger(n) && n >= 1 && n <= 65535) set({ port: n }); }} />
            <button onClick={() => { setPortText('8777'); set({ port: 8777 }); }} disabled={portText === '8777'}>{t('Default')}</button>
          </div>
        </div>
        {!portOK && <div style={{ color: 'var(--danger, #c0392b)', marginTop: 4 }}>{t('Enter a port between 1 and 65535.')}</div>}
        <label className="chk" style={{ marginTop: 8 }}>
          <input type="checkbox" checked={remote.public} onChange={(e) => set({ public: e.target.checked })} />
          {t('Enable Public Access')}
        </label>
        <div className="muted" style={{ marginTop: 4 }}>{t('Off: only this computer can connect (127.0.0.1). On: other devices on the network can connect too (0.0.0.0).')}</div>
        {remote.public && <div style={{ marginTop: 4, color: 'var(--warning, #b9770e)' }}>{t('Anyone on your network will be able to open Word Projector and control the presentation. There is no password.')}</div>}
      </fieldset>
      <div className="muted">{t('Changes take effect the next time Word Projector starts. If the port is taken then, you are asked whether to connect to the running one, use the next free port, or quit.')}</div>
      {info && (
        <fieldset className="group">
          <legend>{t('Right now')}</legend>
          <div>{t('Listening on {addr}', { addr: info.listen })}</div>
          {info.urls.length > 0 && <div style={{ marginTop: 4 }}>{t('Open from other devices:')} {info.urls.map((u) => <a key={u} href={u} target="_blank" rel="noreferrer" style={{ marginRight: 10 }}>{u}</a>)}</div>}
        </fieldset>
      )}
    </div>
  );
}
