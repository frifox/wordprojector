import { useEffect, useRef, useState } from 'react';
import Modal, { alertDialog, confirmDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { BibleInfo, Module, Songbook } from '../lib/types';
import { pickFiles, downloadURL } from '../lib/files';
import NameInfoDialog from './NameInfoDialog';

type Tab = 'bibles' | 'songbooks';

/** The address of the original module list of each kind of book. */
const DEFAULT_LIST_URL: Record<string, string> = {
  bibles: 'http://softprojector.org/bibles/bible.xml',
  songbooks: 'http://softprojector.org/songbooks/songbooks.xml',
};

export default function ManageDataDialog({ onClose, initialTab }: { onClose: () => void; initialTab?: Tab }) {
  const { boot, dataVersion, reloadSettings, settings } = useStore();
  const [tab, setTab] = useState<Tab>(initialTab || 'bibles');
  const [bibles, setBibles] = useState<BibleInfo[]>([]);
  const [songbooks, setSongbooks] = useState<Songbook[]>([]);
  const [sel, setSel] = useState<Record<Tab, number>>({ bibles: 0, songbooks: 0 });
  const [edit, setEdit] = useState<{ kind: Tab; mode: 'new' | 'edit' } | null>(null);
  const [modules, setModules] = useState<{ kind: Tab; list: Module[] | null; msg?: string } | null>(null); // list is null while it is being fetched
  // where the module lists come from: softprojector or custom (an address the user types)
  const [source, setSource] = useState<string>(() => { try { const v = localStorage.getItem('wp-module-source'); return v === 'custom' ? v : 'softprojector'; } catch { return 'softprojector'; } });
  // the address typed for Custom, one for each kind of book; it starts as the address of the softprojector list
  const [customUrls, setCustomUrls] = useState<Record<string, string>>(() => {
    const out: Record<string, string> = {};
    for (const k of Object.keys(DEFAULT_LIST_URL)) {
      let v = '';
      try { v = localStorage.getItem('wp-module-url-' + k) || ''; } catch { /* not remembered */ }
      out[k] = v || DEFAULT_LIST_URL[k];
    }
    return out;
  });
  const listRequest = useRef(0);
  const [modSel, setModSel] = useState<Set<number>>(new Set());

  const load = () => {
    api.bibles().then(setBibles);
    api.songbooks().then(setSongbooks);
  };
  useEffect(load, [dataVersion.bibles, dataVersion.songbooks, dataVersion.all]);

  const defaultSongbook = settings.app.defaultSongbook || 0;
  const toggleDefault = async () => {
    const s2 = JSON.parse(JSON.stringify(settings));
    s2.app.defaultSongbook = defaultSongbook === sel.songbooks ? 0 : sel.songbooks;
    await api.saveSettings(s2);
    reloadSettings();
  };
  const current = sel[tab];
  const select = (id: number) => setSel((s) => ({ ...s, [tab]: id }));
  const kindOf: Record<Tab, 'bible' | 'songbook'> = { bibles: 'bible', songbooks: 'songbook' };
  const ext: Record<Tab, string> = { bibles: '.spb', songbooks: '.sps' };
  const nameOf = (): string => {
    if (tab === 'bibles') return bibles.find((b) => b.id === current)?.name || '';
    return songbooks.find((b) => b.id === current)?.name || '';
  };

  const doImport = async () => {
    const paths = await pickFiles(boot.desktop, t('Select a file to import'), [{ name: `SoftProjector file (*${ext[tab]})`, pattern: `*${ext[tab]}` }], true);
    if (!paths.length) return;
    try {
      await api.importFiles(kindOf[tab], paths);
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
  };
  const doExport = async () => {
    if (!current) return;
    try {
      const r = await api.exportFile(kindOf[tab], { id: current, name: nameOf() });
      if (r.cancelled) return;
      if (r.download) downloadURL(r.download);
      else await alertDialog(t('Export complete'), t('"{name}" has been saved to:\n{path}', { name: nameOf(), path: r.path || '' }));
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
  };
  const doDelete = async () => {
    if (!current) return;
    const name = nameOf();
    const msg = tab === 'bibles' ? t('Delete Bible?') : t('Delete songbook?');
    const info = tab === 'bibles' ? t('This action will permanently delete this Bible') : t('This action will permanently delete this songbook');
    const r = await confirmDialog({ title: msg, message: t('Are you sure that you want to delete: ') + name, info, buttons: [{ label: t('Yes'), value: 'yes', primary: true }, { label: t('No'), value: 'no' }] });
    if (r !== 'yes') return;
    if (tab === 'bibles') await api.deleteBible(current);
    else await api.deleteSongbook(current);
    select(0);
    load();
    reloadSettings();
  };
  const fetchModules = async (kind: Tab, src: string, url: string) => {
    // the list area shows a spinner while the list is on its way
    const id = ++listRequest.current;
    setModSel(new Set());
    if (src === 'custom' && !url.trim()) {
      setModules({ kind, list: [], msg: t('Enter the web address of a module site, then press Load.') });
      return;
    }
    setModules({ kind, list: null });
    try {
      const list = await api.moduleList(kindOf[kind], src, url);
      if (listRequest.current === id) setModules({ kind, list });
    } catch (e) {
      if (listRequest.current === id) setModules({ kind, list: [], msg: String((e as Error).message || e) });
    }
  };
  const doDownload = () => fetchModules(tab, source, customUrls[tab]);
  const pickSource = (src: string) => {
    setSource(src);
    try { localStorage.setItem('wp-module-source', src); } catch { /* not remembered */ }
    if (modules) fetchModules(modules.kind, src, customUrls[modules.kind]);
  };
  const loadCustom = () => {
    if (!modules) return;
    try { localStorage.setItem('wp-module-url-' + modules.kind, customUrls[modules.kind]); } catch { /* not remembered */ }
    fetchModules(modules.kind, 'custom', customUrls[modules.kind]);
  };
  const closeModules = () => { listRequest.current++; setModules(null); };
  const startDownload = async () => {
    if (!modules?.list) return;
    const mods = modules.list.filter((_, i) => modSel.has(i));
    setModules(null);
    if (!mods.length) return;
    try {
      await api.moduleDownload(kindOf[modules.kind], mods);
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
  };

  const rows: { id: number; a: string; b?: string; abbr?: string; star?: boolean }[] =
    tab === 'bibles' ? bibles.map((b) => ({ id: b.id, a: b.name, abbr: b.abbr })) : songbooks.map((b) => ({ id: b.id, a: b.name, star: b.id === defaultSongbook, b: b.info }));
  const count = rows.length;

  return (
    <>
      <Modal title={t('Books')} onClose={onClose} width={720} height={520} buttons={<button className="primary" onClick={onClose}>{t('Close')}</button>}>
        <div className="tabs fill">
          <div className="tabbar">
            {(['bibles', 'songbooks'] as Tab[]).map((tb) => (
              <div key={tb} className={`tab ${tab === tb ? 'active' : ''}`} onClick={() => setTab(tb)}>{tb === 'bibles' ? t('Bibles') : t('Songbooks')}</div>
            ))}
          </div>
          <div className="tabpage">
            <div className="row grow" style={{ alignItems: 'stretch', minHeight: 0 }}>
              <div className="col grow" style={{ minWidth: 0, minHeight: 0 }}>
              <div className="table grow" tabIndex={0}>
                <table>
                  <thead><tr><th>{t('Title')}</th>{tab === 'bibles' && <th style={{ width: 110 }}>{t('Abbreviation')}</th>}{tab !== 'bibles' && <th>{t('Information')}</th>}</tr></thead>
                  <tbody>
                    {rows.map((r) => (
                      <tr key={r.id} className={r.id === current ? 'sel' : ''} onClick={() => select(r.id)} onDoubleClick={() => setEdit({ kind: tab, mode: 'edit' })}>
                        <td>{r.star && <span title={t('Default')} style={{ color: '#e6a100', marginRight: 6 }}>★</span>}{r.a}</td>{tab === 'bibles' && <td>{r.abbr}</td>}{tab !== 'bibles' && <td>{(r.b || '').split('\n')[0]}</td>}
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
              <div className="books-bar">
                <button className="small" onClick={doDownload}>{t('Import from Cloud')}</button>
                <button className="small" onClick={doImport}>{t('Import from File')}</button>
                {tab === 'songbooks' && <button className="small" disabled={!current} onClick={toggleDefault}>{current && current === defaultSongbook ? t('Unset as Default') : t('Set as Default')}</button>}
                <span className="grow" />
                <button className="small" disabled={!current} onClick={() => setEdit({ kind: tab, mode: 'edit' })}>{t('Edit')}</button>
                <button className="small" disabled={!current || count < 2} onClick={doDelete}>{t('Delete')}</button>
                <button className="small" disabled={!current} onClick={doExport}>{t('Export')}</button>
              </div>
              </div>
            </div>
          </div>
        </div>
      </Modal>
      {edit && edit.kind === 'bibles' && (
        <BibleInfoDialog bible={bibles.find((b) => b.id === current)!} onClose={() => setEdit(null)} onSaved={() => { setEdit(null); load(); reloadSettings(); }} />
      )}
      {edit && edit.kind === 'songbooks' && (
        <NameInfoDialog title={t('Edit Songbook')} nameLabel={t('Songbook Name:')} infoLabel={t('Information:')} name={songbooks.find((b) => b.id === current)?.name} info={songbooks.find((b) => b.id === current)?.info}
          onClose={() => setEdit(null)} onSave={async (n, i) => { await api.updateSongbook(current, n, i); setEdit(null); load(); reloadSettings(); }} />
      )}
      {modules && (
        <Modal title={modules.kind === 'bibles' ? t('Bible Module Download') : t('Songbook Module Download')} onClose={closeModules} width={460} height={440}
          buttons={<><button onClick={closeModules}>{t('Cancel')}</button><button className="primary" onClick={startDownload} disabled={!modSel.size}>{t('OK')}</button></>}>
          <div className="row">
            <span>{t('Source:')}</span>
            <select value={source} onChange={(e) => pickSource(e.target.value)}>
              <option value="softprojector">softprojector.org</option>
              <option value="custom">{t('Custom')}</option>
            </select>
            {source === 'custom' && (
              <>
                <input type="text" className="grow" style={{ minWidth: 0 }} placeholder="http://example.org" value={customUrls[modules.kind]} onChange={(e) => setCustomUrls((u) => ({ ...u, [modules.kind]: e.target.value }))}
                  onKeyDown={(e) => { if (e.key === 'Enter') { e.preventDefault(); loadCustom(); } }} autoFocus />
                <button onClick={loadCustom} disabled={!customUrls[modules.kind].trim()}>{t('Load')}</button>
              </>
            )}
          </div>
          <div className="muted">{t('Select the modules to download and import:')}</div>
          <div className="list grow" tabIndex={0}>
            {modules.list === null && <div style={{ display: 'flex', justifyContent: 'center', padding: 24 }}><div className="bg-spinner" role="status" aria-label={t('Loading…')} /></div>}
            {modules.list?.map((m, i) => (
              <div key={i} className={`item ${modSel.has(i) ? 'sel' : ''}`} onClick={() => setModSel((s) => { const n = new Set(s); if (n.has(i)) n.delete(i); else n.add(i); return n; })}>
                {m.name} <span className="muted">({Math.round(m.size / 1024)} kB)</span>
              </div>
            ))}
            {modules.list && !modules.list.length && <div className="item muted" style={{ whiteSpace: 'normal' }}>{modules.msg || t('No modules available.')}</div>}
          </div>
          <div className="row">
            <button disabled={!modules.list} onClick={() => setModSel(new Set((modules.list ?? []).map((_, i) => i)))}>{t('Select All')}</button>
            <button onClick={() => setModSel(new Set())}>{t('Deselect All')}</button>
          </div>
        </Modal>
      )}
    </>
  );
}

function BibleInfoDialog({ bible, onClose, onSaved }: { bible: BibleInfo; onClose: () => void; onSaved: () => void }) {
  const [b, setB] = useState<BibleInfo>({ ...bible });
  const requestClose = useDiscardGuard(JSON.stringify(b) !== JSON.stringify(bible), onClose, () => saveBible());
  const saveBible = async () => { await api.updateBible(b); onSaved(); };
  return (
    <Modal title={t('Bible Information')} onClose={requestClose} width={460} buttons={<><button onClick={requestClose}>{t('Cancel')}</button><button className="primary" onClick={saveBible}>{t('OK')}</button></>}>
      <div className="form-grid">
        <span>{t('Title:')}</span><input type="text" autoFocus value={b.name} onChange={(e) => setB({ ...b, name: e.target.value })} />
        <span>{t('Abbreviation:')}</span><input type="text" value={b.abbr} onChange={(e) => setB({ ...b, abbr: e.target.value })} />
        <span style={{ alignSelf: 'flex-start' }}>{t('Information:')}</span><textarea rows={5} value={b.info} onChange={(e) => setB({ ...b, info: e.target.value })} />
        <span />
        <label className="chk"><input type="checkbox" checked={b.rtl} onChange={(e) => setB({ ...b, rtl: e.target.checked })} />{t('Right to left text')}</label>
      </div>
    </Modal>
  );
}
