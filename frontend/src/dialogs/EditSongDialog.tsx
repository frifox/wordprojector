import { useEffect, useRef, useState } from 'react';
import Modal, { alertDialog, promptDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { Song, Songbook } from '../lib/types';
import { FontPicker, ColorField, AlignGroup, HighlightTextarea } from '../components/Pickers';
import { pickFiles, imageFilter } from '../lib/files';
import { useUI } from '../lib/ui';
import NameInfoDialog from './NameInfoDialog';

const defaultFont = { family: 'Arial', pointSize: 28, bold: false, italic: false };

function blankSong(): Song {
  return {
    id: 0, songbookId: 0, songbookName: '', number: 1, title: '', category: 0, tune: '', wordsBy: '', musicBy: '',
    text: t('Verse 1\n - words of verse go here\n\nRefrain\n- words of Chorus/Refrain\ngo here\n\nVerse 2\n - words of verse go here'),
    notes: '', usePrivate: false, alignV: 1, alignH: 1, color: '#ffffff', font: { ...defaultFont }, infoColor: '#ffffff', infoFont: { ...defaultFont },
    endingColor: '#ffffff', endingFont: { ...defaultFont }, useBackground: false, backgroundName: '', hasBackground: false, count: 0, date: '',
  };
}

export default function EditSongDialog({ mode, song, onClose }: { mode: 'new' | 'edit' | 'copy'; song?: Song; onClose: () => void }) {
  const { songbooks, boot, reloadSettings } = useStore();
  const ui = useUI();
  const [s, setS] = useState<Song>(() => (song ? { ...song, text: song.text || '' } : blankSong()));
  const [isNew, setIsNew] = useState(mode !== 'edit');
  const [ready, setReady] = useState(mode === 'edit');
  const [bgPath, setBgPath] = useState('');
  const [clearBg, setClearBg] = useState(false);
  const [addSongbook, setAddSongbook] = useState(false);
  // the state the editor opened with; taken once the song is ready to edit
  const baseline = useRef<string | null>(null);
  if (ready && baseline.current === null) baseline.current = JSON.stringify(s);
  const dirty = baseline.current !== null && (JSON.stringify(s) !== baseline.current || bgPath !== '' || clearBg);
  const requestClose = useDiscardGuard(dirty, onClose, () => save());
  const [, setBooks] = useState<Songbook[]>(songbooks);
  const categories = boot.categories.map((name, id) => ({ id, name: t(name) })).sort((a, b) => a.name.localeCompare(b.name));
  const titles = [...boot.titles.verse, ...boot.titles.verse.map((w) => '&' + w), ...boot.titles.refrain, ...boot.titles.refrain.map((w) => '&' + w), ...boot.titles.slide];

  const set = (p: Partial<Song>) => setS((x) => ({ ...x, ...p }));

  // On open: pick the songbook (new/copy) or verify the edited song exists.
  useEffect(() => {
    (async () => {
      if (mode === 'edit' && song) {
        if (song.id > 0) {
          const full = song.text !== undefined ? song : await api.song(song.id);
          setS({ ...full, text: full.text || '' });
          setReady(true);
          return;
        }
        const m = await api.matchSong(song.songbookName, song.number, song.title);
        if (m.found) {
          const full = await api.song(m.id);
          setS({ ...full });
          setReady(true);
          return;
        }
        setIsNew(true);
        await chooseSongbook(t('Cannot find exact match in database'), t('The exact match of a song you are editing was not found in database.\nIn order to edit this song, you need to add it to database.\n\nPlease select a Songbook to which you want to copy this song to:'));
        return;
      }
      if (mode === 'copy') await chooseSongbook(t('Copy to a new Songbook'), t('Select a Songbook to which you want to copy this song to'));
      else await chooseSongbook(t('Add a new Songbook'), t('Select a Songbook to which you want to add a song'));
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const chooseSongbook = async (newLabel: string, caption: string) => {
    const list = await api.songbooks();
    setBooks(list);
    const options = [newLabel, ...list.map((b) => b.name)];
    const choice = await promptDialog({ title: t('Select Songbook'), message: caption, options, initial: Math.max(0, list.findIndex((b) => b.id === s.songbookId) + 1) });
    if (choice === null) {
      onClose();
      return;
    }
    if (choice === newLabel) {
      setAddSongbook(true);
      return;
    }
    const sb = list.find((b) => b.name === choice)!;
    const n = await api.nextNumber(sb.id);
    set({ songbookId: sb.id, songbookName: sb.name, number: n.number, id: mode === 'edit' ? s.id : 0 });
    setReady(true);
  };

  const onSongbookAdded = async (name: string, info: string) => {
    const r = await api.addSongbook(name, info);
    await reloadSettings();
    setAddSongbook(false);
    const n = await api.nextNumber(r.id);
    set({ songbookId: r.id, songbookName: name, number: n.number });
    setReady(true);
  };

  const save = async () => {
    const title = s.title.trim().replace(/\s+/g, ' ');
    if (!title) {
      await alertDialog(t('Song title is missing'), t('Song title cannot be left empty.\nPlease enter song title.'));
      return;
    }
    const song: Song = { ...s, title, text: (s.text || '').trim() };
    try {
      if (isNew) await api.createSong({ ...song, id: 0 }, bgPath);
      else await api.updateSong(song, bgPath, clearBg);
    } catch (e) {
      await alertDialog(t('Error'), String((e as Error).message || e));
      return;
    }
    onClose();
  };

  const browseBg = async () => {
    const paths = await pickFiles(boot.desktop, t('Select an image for the wallpaper'), [imageFilter], false);
    if (paths[0]) {
      setBgPath(paths[0]);
      setClearBg(false);
      set({ backgroundName: paths[0].split(/[\\/]/).pop() || '', hasBackground: true });
    }
  };

  const print = () => {
    let text = `${s.songbookName} ${s.number}\n${s.title}\n`;
    if (s.tune) text += t('Tune: {tune}', { tune: s.tune }) + '\n';
    text += '\n' + (s.text || '');
    ui.openDialog({ kind: 'print', title: t('Print'), text, fontSize: 14 });
  };

  if (addSongbook) {
    return <NameInfoDialog title={t('Add a Songbook')} nameLabel={t('Songbook Name:')} infoLabel={t('Information:')} onSave={onSongbookAdded} onClose={onClose} />;
  }
  if (!ready) return null;

  return (
    <Modal
      title={t('Edit and/or Add New songs')}
      onClose={requestClose}
      width={760}
      height="92vh"
      buttons={
        <>
          <button onClick={print}><img className="icon" src="/icons/print.png" alt="" />{t('Print')}</button>
          <span className="grow" />
          <button onClick={requestClose}>{t('Cancel')}</button>
          <button className="primary" onClick={save}>{t('Save')} <span className="kbd">Ctrl+S</span></button>
        </>
      }
    >
      <div onKeyDown={(e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); save(); } }} className="col fill" style={{ gap: 6 }}>
        <div className="row">
          <span>{t('Songbook:')}</span>
          <b>{s.songbookName}</b>
          <input type="number" min={1} value={s.number} style={{ width: 80 }} onChange={(e) => set({ number: Number(e.target.value) || 1 })} />
          <span className="grow" />
        </div>
        <div className="form-grid">
          <span>{t('Title:')}</span>
          <input type="text" value={s.title} autoFocus onChange={(e) => set({ title: e.target.value })} />
          <span>{t('Words by:')}</span>
          <div className="row">
            <input type="text" className="grow" value={s.wordsBy} onChange={(e) => set({ wordsBy: e.target.value })} />
            <span>{t('Music by:')}</span>
            <input type="text" className="grow" value={s.musicBy} onChange={(e) => set({ musicBy: e.target.value })} />
          </div>
          <span>{t('Key:')}</span>
          <div className="row">
            <input type="text" value={s.tune} style={{ width: 120 }} onChange={(e) => set({ tune: e.target.value })} />
            <span>{t('Category:')}</span>
            <select className="grow" value={s.category} onChange={(e) => set({ category: Number(e.target.value) })}>
              {categories.map((c) => (
                <option key={c.id} value={c.id}>{c.name}</option>
              ))}
            </select>
          </div>
        </div>
        <label className="chk"><input type="checkbox" checked={s.usePrivate} onChange={(e) => set({ usePrivate: e.target.checked })} />{t('Use Private Song Settings')}</label>
        {s.usePrivate && (
          <fieldset className="group col" style={{ gap: 6 }}>
            <div className="row"><span style={{ width: 190 }}>{t('Main Text Properties:')}</span><ColorField value={s.color} onChange={(c) => set({ color: c })} /><FontPicker value={s.font} onChange={(f) => set({ font: f })} /></div>
            <div className="row"><span style={{ width: 190 }}>{t('Main Text Alignment:')}</span><AlignGroup axis="v" h={s.alignH} value={s.alignV} onChange={(v) => set({ alignV: v })} /><AlignGroup axis="h" value={s.alignH} onChange={(v) => set({ alignH: v })} /></div>
            <div className="row"><span style={{ width: 190 }}>{t('Song Information Properties:')}</span><ColorField value={s.infoColor} onChange={(c) => set({ infoColor: c })} /><FontPicker value={s.infoFont} onChange={(f) => set({ infoFont: f })} /></div>
            <div className="row"><span style={{ width: 190 }}>{t('Song Ending Properties:')}</span><ColorField value={s.endingColor} onChange={(c) => set({ endingColor: c })} /><FontPicker value={s.endingFont} onChange={(f) => set({ endingFont: f })} /></div>
            <div className="row">
              <label className="chk" style={{ width: 190 }}><input type="checkbox" checked={s.useBackground} onChange={(e) => set({ useBackground: e.target.checked })} />{t('Use Background:')}</label>
              <input type="text" className="grow" readOnly value={s.backgroundName} disabled={!s.useBackground} />
              <button disabled={!s.useBackground} onClick={browseBg}>{t('Browse...')}</button>
              <button disabled={!s.useBackground || !s.hasBackground} onClick={() => { setClearBg(true); setBgPath(''); set({ backgroundName: '', hasBackground: false }); }}>{t('Remove')}</button>
            </div>
          </fieldset>
        )}
        <HighlightTextarea value={s.text || ''} onChange={(v) => set({ text: v })} titleWords={titles} style={{ flex: 1, minHeight: 200 }} />
        <div className="row" style={{ alignItems: 'flex-start' }}>
          <span>{t('Notes:')}</span>
          <textarea className="grow" rows={2} value={s.notes} onChange={(e) => set({ notes: e.target.value })} />
        </div>
      </div>
    </Modal>
  );
}
