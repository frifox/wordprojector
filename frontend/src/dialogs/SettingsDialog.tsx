import { useEffect, useRef, useState } from 'react';
import Modal, { alertDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { useT } from '../lib/hooks';
import { t, languages, setLanguage, getLanguage } from '../i18n';
import type { BibleSettings, Settings, SongSettings, TextSettings, Screen } from '../lib/types';
import { FontPicker, FontStyleButtons, ColorField, Select, AlignGroup, IconChoice, POSITION_ICONS } from '../components/Pickers';
import { UserInterfaceSection } from './UiSections';
import UpdatesPage from './UpdatesPage';
import HotkeysPage from './HotkeysPage';
import ScreenMap from '../components/ScreenMap';
import { CountdownDisc } from '../projector/SlideView';

type Page = 'general' | 'display' | 'hotkeys' | 'remote' | 'updates';

export default function SettingsDialog({ onClose, initialPage }: { onClose: () => void; initialPage?: Page }) {
  const store = useStore();
  const [page, setPage] = useState<Page>(initialPage ?? 'general');
  const [s, setS] = useState<Settings>(() => JSON.parse(JSON.stringify(store.settings)));
  const [screens, setScreens] = useState<Screen[]>(store.boot.screens || []);
  const baseline = useRef(JSON.stringify(store.settings));
  const dirty = JSON.stringify(s) !== baseline.current;
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
  // `next` lets a caller apply a change that is not in state yet (double clicking a theme thumbnail)
  const apply = async (next: Settings = s) => {
    try {
      await api.saveSettings(next);
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

  const pages: { id: Page; label: string }[] = [
    { id: 'general', label: t('General') }, { id: 'display', label: t('Display') }, { id: 'hotkeys', label: t('Hotkeys') },
    { id: 'remote', label: t('Remote') },
    { id: 'updates', label: t('Updates') },
  ];
  const headers: Record<Page, string> = { general: t('General Word Projector Settings'), display: t('Display Settings'), hotkeys: t('Global Hotkeys'), remote: t('Remote Access Settings'), updates: t('Updates') };

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
          {page === 'general' && <GeneralPage s={s} setS={setS} applyNow={(next) => { setS(next); return apply(next); }} />}
          {page === 'display' && <DisplayPage s={s} setS={setS} setGeneral={setGeneral} screens={screens} />}
          {page === 'hotkeys' && <HotkeysPage s={s} setS={setS} />}
          {page === 'remote' && <RemotePage s={s} setS={setS} />}
          {page === 'updates' && <UpdatesPage s={s} setS={setS} />}
        </div>
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

/** Settings > General: the look of the program and its language. */
function GeneralPage({ s, setS, applyNow }: { s: Settings; setS: (f: (x: Settings) => Settings) => void; applyNow: (next: Settings) => Promise<void> }) {
  useT(); // the language buttons follow the chosen language
  const { reloadSettings } = useStore();
  const chosen = getLanguage();
  const pick = (code: string) => {
    setLanguage(code);
    api.patchApp({ language: code }).then(() => reloadSettings()).catch(() => {});
  };
  return (
    <div className="col" style={{ gap: 4 }}>
      <UserInterfaceSection s={s} setS={setS} applyNow={applyNow} />
      <fieldset className="group">
        <legend>{t('Language')}</legend>
        <div className="row" style={{ gap: 12, alignItems: 'flex-start', flexWrap: 'wrap' }}>
          {languages.map((l) => (
            <button key={l.code} className={`ui-thumb ${chosen === l.code ? 'on' : ''}`} onClick={() => pick(l.code)} aria-pressed={chosen === l.code}>
              <img src={`/icons/${l.flag}`} alt="" width={32} height={32} draggable={false} />
              <span>{l.name}</span>
            </button>
          ))}
        </div>
      </fieldset>
    </div>
  );
}

/** Settings > Display: which screens show the presentation, and the on-screen controls. */
function DisplayPage({ s, setS, setGeneral, screens }: { s: Settings; setS: (f: (x: Settings) => Settings) => void; setGeneral: (p: Partial<Settings['general']>) => void; screens: Screen[] }) {
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
      <fieldset className="group">
        <legend>{t('Preview Display')}</legend>
        <label className="chk"><input type="checkbox" checked={s.app.livePreview} onChange={(e) => setS((x) => ({ ...x, app: { ...x.app, livePreview: e.target.checked } }))} />{t('Show the preview above the live list')}</label>
      </fieldset>
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
