import { useEffect, useRef, useState } from 'react';
import BackgroundSection from './BackgroundSection';
import Modal, { alertDialog, confirmDialog } from '../components/Modal';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { RenderSlide, Settings, Theme, ThemeInfo } from '../lib/types';
import NameInfoDialog from './NameInfoDialog';
import { ColorField } from '../components/Pickers';
import SlidePreview, { TransitionPreview, type PresentControl } from './SlidePreview';
import { TransitionSection } from './UiSections';
import { BiblePage, SongPage } from './SettingsDialog';

type Tab = 'general' | 'bible' | 'songs';

const Pencil = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4" /><path d="M13.5 6.5l4 4" /></svg>;
const Plus = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5l0 14" /><path d="M5 12l14 0" /></svg>;
const Copy = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M8 8m0 2a2 2 0 0 1 2 -2h8a2 2 0 0 1 2 2v8a2 2 0 0 1 -2 2h-8a2 2 0 0 1 -2 -2z" /><path d="M16 8v-2a2 2 0 0 0 -2 -2h-8a2 2 0 0 0 -2 2v8a2 2 0 0 0 2 2h2" /></svg>;
const Trash = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>;

const clone = <T,>(x: T): T => JSON.parse(JSON.stringify(x));

/**
 * The theme editor: every theme on the left, the selected one on the right as a form with a live preview.
 * Changes are kept per theme while you click around (themes with unsaved changes carry a dot) and are only
 * written when you save; leaving with unsaved themes asks first.
 */
export default function ThemeEditor({ s, setS, currentId, onUse, onClose }: {
  s: Settings; setS: (f: (x: Settings) => Settings) => void; currentId: number;
  /** makes a theme the one in use */
  onUse: (id: number) => void; onClose: () => void;
}) {
  const [list, setList] = useState<ThemeInfo[]>([]);
  const [sel, setSel] = useState(currentId);
  const [tab, setTab] = useState<Tab>('general');
  const [base, setBase] = useState<Record<number, Theme>>({}); // as saved
  const [draft, setDraft] = useState<Record<number, Theme>>({}); // as edited
  const [naming, setNaming] = useState<null | 'add' | 'copy' | 'saveAs' | number>(null); // a number renames that theme
  const [inUse, setInUse] = useState(currentId);
  // a font under the pointer in a font list: the preview shows it before it is chosen (nothing is changed)
  const [hover, setHover] = useState<{ tab: Tab; patch: object } | null>(null);

  // the Preview button: the sample on the previews goes to the real displays until it is turned off or the editor closes
  const [presenting, setPresenting] = useState(false);
  const lastSlide = useRef<RenderSlide | null>(null);
  const present: PresentControl = {
    on: presenting,
    toggle: () => setPresenting((on) => !on),
    send: (slide) => { lastSlide.current = slide; api.previewSlide(slide).catch(() => {}); },
  };
  useEffect(() => {
    if (!presenting) return;
    const keep = window.setInterval(() => { if (lastSlide.current) api.previewSlide(lastSlide.current).catch(() => {}); }, 10000);
    return () => {
      window.clearInterval(keep);
      api.clearPreview().catch(() => {});
    };
  }, [presenting]);

  const reloadList = async () => setList(await api.themes());
  useEffect(() => { reloadList(); }, []);
  useEffect(() => {
    if (base[sel]) return;
    let live = true;
    api.theme(sel).then((th) => { if (live) { setBase((b) => ({ ...b, [sel]: th })); setDraft((d) => ({ ...d, [sel]: clone(th) })); } }).catch(() => {});
    return () => { live = false; };
  }, [sel, base]);

  const dirtyOf = (id: number) => !!base[id] && !!draft[id] && JSON.stringify(base[id]) !== JSON.stringify(draft[id]);
  const dirtyIds = list.filter((x) => dirtyOf(x.id)).map((x) => x.id);
  const info = list.find((x) => x.id === sel);
  const th = draft[sel];
  const dirty = dirtyOf(sel);

  const upd = (fn: (t: Theme) => void) => setDraft((d) => {
    const n = clone(d[sel]);
    fn(n);
    return { ...d, [sel]: n };
  });
  const fail = (e: unknown) => alertDialog(t('Error'), String((e as Error).message || e));
  const forget = (id: number) => {
    setBase((b) => { const n = { ...b }; delete n[id]; return n; });
    setDraft((d) => { const n = { ...d }; delete n[id]; return n; });
  };

  const requestExit = async () => {
    if (dirtyIds.length) {
      const names = list.filter((x) => dirtyIds.includes(x.id)).map((x) => `"${x.name}"`).join(', ');
      const r = await confirmDialog({
        title: t('Exit without saving?'),
        message: t('These themes have unsaved changes: {names}.', { names }),
        info: t('The changes will be lost.'),
        buttons: [{ label: t('Exit Without Saving'), value: 'exit', primary: true }, { label: t('Go Back'), value: 'back' }],
      });
      if (r !== 'exit') return;
    }
    onClose();
  };

  const revert = () => setDraft((d) => ({ ...d, [sel]: clone(base[sel]) }));
  const save = async () => {
    try {
      await api.saveTheme(th);
      setBase((b) => ({ ...b, [sel]: clone(th) }));
    } catch (e) { fail(e); }
  };
  // pictures are added and removed at once, outside of Save: take over what the server chose (the first picture of an empty
  // pool is chosen, a removed one is replaced), but keep the other unsaved changes of the draft
  const onPool = async (ids: number[]) => {
    const id = sel;
    try {
      const fresh = await api.theme(id);
      setBase((b) => (b[id] ? { ...b, [id]: { ...b[id], passive: fresh.passive } } : b));
      setDraft((d) => {
        if (!d[id]) return d;
        const p = d[id].passive;
        const gone = p.imageId > 0 && !ids.includes(p.imageId);
        const empty = p.imageId === 0 && fresh.passive.imageId > 0;
        return { ...d, [id]: { ...d[id], passive: gone || empty ? { ...p, imageId: fresh.passive.imageId, useBackground: fresh.passive.useBackground || p.useBackground && fresh.passive.imageId > 0 } : p } };
      });
    } catch (e) { fail(e); }
  };
  // a new theme from the settings as they are on screen (changes not saved yet included)
  const copyFrom = async (name: string, from: number) => {
    const r = await api.copyTheme(from, name);
    const src = draft[from];
    // the copy has its own pictures: point the choice at the copy's one
    const own = await api.theme(r.id);
    const made = { ...clone(src), id: r.id, name: name.trim(), passive: { ...src.passive, imageId: src.passive.imageId > 0 ? own.passive.imageId : 0 } };
    await api.saveTheme(made);
    await reloadList();
    setBase((b) => ({ ...b, [r.id]: clone(made) }));
    setDraft((d) => ({ ...d, [r.id]: clone(made) }));
    return r.id;
  };
  const onName = async (name: string) => {
    const what = naming;
    try {
      if (what === 'add') {
        const r = await api.createTheme(name);
        await reloadList();
        setSel(r.id);
      } else if (what === 'copy') {
        setSel(await copyFrom(name, sel));
      } else if (what === 'saveAs') {
        const id = await copyFrom(name, sel);
        revert(); // the changes moved to the new theme
        setSel(id);
      } else if (typeof what === 'number') {
        await api.renameTheme(what, name);
        await reloadList();
        const n = name.trim();
        setBase((b) => (b[what] ? { ...b, [what]: { ...b[what], name: n } } : b));
        setDraft((d) => (d[what] ? { ...d, [what]: { ...d[what], name: n } } : d));
      }
      setNaming(null);
    } catch (e) { fail(e); }
  };
  const remove = async () => {
    if (!info || info.builtin) return;
    const r = await confirmDialog({
      title: t('Delete Theme?'),
      message: t('Are you sure that you want to delete: ') + info.name,
      info: t('This action will permanently delete this theme'),
      buttons: [{ label: t('Yes'), value: 'yes', primary: true }, { label: t('No'), value: 'no' }],
    });
    if (r !== 'yes') return;
    try {
      await api.deleteTheme(sel);
      const rest = (await api.themes());
      setList(rest);
      forget(sel);
      const next = rest[0]?.id ?? 0;
      if (sel === inUse && next) { setInUse(next); onUse(next); }
      setSel(next);
    } catch (e) { fail(e); }
  };

  // the text effects are one choice for the Bible and the songs
  const setEffect = (p: Partial<{ useShadow: boolean; useBlurShadow: boolean }>) => upd((tm) => {
    tm.bible = { ...tm.bible, ...p };
    tm.song = { ...tm.song, ...p };
  });

  const body = () => {
    if (tab === 'general') {
      return (
        <div className="theme-editor-split">
          <div className="theme-editor-form">
            <fieldset className="group"><legend>{t('Background Color')}</legend>
              <div className="row" style={{ gap: 8 }}>
                <ColorField bare value={th.backgroundColor || '#000000'} onChange={(c) => upd((tm) => { tm.backgroundColor = c; })} />
                <span className="muted">{th.backgroundColor || '#000000'}</span>
              </div>
            </fieldset>
            <TransitionSection s={s} setS={setS} />
            <fieldset className="group"><legend>{t('Text Effects')}</legend>
              <div className="row" style={{ gap: 16 }}>
                <label className="chk"><input type="checkbox" checked={th.bible.useShadow} onChange={(e) => setEffect({ useShadow: e.target.checked })} />{t('Use shadow')}</label>
                <label className="chk"><input type="checkbox" checked={th.bible.useBlurShadow} onChange={(e) => setEffect({ useBlurShadow: e.target.checked })} />{t('Use blurred shadow')}</label>
              </div>
            </fieldset>
            <BackgroundSection themeId={sel} v={th.passive} onChange={(p) => upd((tm) => { tm.passive = { ...tm.passive, ...p }; })} onPool={onPool} />
          </div>
          <div className="theme-editor-preview"><TransitionPreview theme={th} settings={s} present={present} /></div>
        </div>
      );
    }
    const kind = tab === 'bible' ? 'bible' : 'song';
    return (
      <div className="theme-editor-split">
        <div className="theme-editor-form">
          {tab === 'bible'
            ? <BiblePage v={th.bible} onChange={(p) => upd((tm) => { tm.bible = { ...tm.bible, ...p }; })} onPreview={(patch) => setHover(patch && { tab, patch })} />
            : <SongPage v={th.song} onChange={(p) => upd((tm) => { tm.song = { ...tm.song, ...p }; })} onPreview={(patch) => setHover(patch && { tab, patch })} />}
        </div>
        <div className="theme-editor-preview"><SlidePreview kind={kind} theme={hover?.tab === tab ? { ...th, [kind]: { ...th[kind], ...hover.patch } } : th} settings={s} present={present} /></div>
      </div>
    );
  };

  return (
    <Modal width={1180} height="min(780px, 92vh)" title={t('Theme Editor')} onClose={requestExit} buttons={<button onClick={requestExit}>{t('Close')}</button>}>
      <div className="theme-editor">
        <div className="theme-editor-side">
          <div className="theme-editor-list">
            {list.map((x) => (
              <div key={x.id} className={`theme-editor-item ${x.id === sel ? 'sel' : ''}`} onClick={() => setSel(x.id)}>
                <span className="theme-editor-name">{x.name}</span>
                {dirtyOf(x.id) && <span className="theme-editor-dot" title={t('Unsaved changes')}>●</span>}
                {x.id === inUse && <span className="theme-editor-badge">{t('Current')}</span>}
                {x.id !== inUse && <button className="theme-editor-use" title={t('Switch to this theme')} onClick={(e) => { e.stopPropagation(); setInUse(x.id); onUse(x.id); }}>{t('Switch')}</button>}
              </div>
            ))}
          </div>
          <div className="theme-editor-tools">
            <button className="tbtn-s" title={t('New Theme')} onClick={() => setNaming('add')}><Plus /></button>
            <button className="tbtn-s" title={t('Duplicate Theme')} disabled={!th} onClick={() => setNaming('copy')}><Copy /></button>
            <button className="tbtn-s" title={info?.builtin ? t('Light and Dark can\'t be renamed') : t('Rename Theme')} disabled={!info || info.builtin} onClick={() => setNaming(sel)}><Pencil /></button>
            <button className="tbtn-s" title={info?.builtin ? t('Light and Dark can\'t be deleted') : t('Delete Theme')} disabled={!info || info.builtin} onClick={remove}><Trash /></button>
          </div>
        </div>
        <div className="theme-editor-card">
          {!th ? <div className="muted" style={{ padding: 12 }}>{t('Loading…')}</div> : (
            <>
              <div className="tabs underline-tabs" style={{ height: 'auto', flex: 1, minHeight: 0 }}>
                <div className="tabbar">
                  {([['general', t('General')], ['bible', t('Bible')], ['songs', t('Songs')]] as [Tab, string][]).map(([id, label]) => (
                    <div key={id} className={`tab ${tab === id ? 'active' : ''}`} onClick={() => setTab(id)}>{label}</div>
                  ))}
                </div>
                <div className="theme-editor-body">{body()}</div>
              </div>
              <div className="row theme-editor-bar">
                <span className={dirty ? '' : 'muted'}>{dirty ? `● ${t('Unsaved changes')}` : t('No changes')}</span>
                <span className="grow" />
                <button onClick={revert} disabled={!dirty}>{t('Revert')}</button>
                <button onClick={() => setNaming('saveAs')}>{t('Save As...')}</button>
                <button className="primary" onClick={save} disabled={!dirty}>{t('Save')}</button>
              </div>
            </>
          )}
        </div>
      </div>
      {naming !== null && (
        <NameInfoDialog title={naming === 'add' ? t('New Theme') : naming === 'copy' ? t('Duplicate Theme') : naming === 'saveAs' ? t('Save Theme As') : t('Rename Theme')} nameLabel={t('Theme Name:')}
          name={naming === 'copy' ? `${info?.name ?? ''} ${t('copy')}` : typeof naming === 'number' ? list.find((x) => x.id === naming)?.name : ''}
          onClose={() => setNaming(null)} onSave={onName} />
      )}
    </Modal>
  );
}
