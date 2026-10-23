import { useEffect, useMemo, useRef, useState } from 'react';
import { phoneticContains, phoneticRange } from '../lib/translit';
import StanzaText from '../components/StanzaText';
import { startScheduleDrag } from '../lib/dnd';
import { focusLive } from '../lib/focus';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import { useRegisterPanel, useUI } from '../lib/ui';
import Splitter from '../components/Splitter';
import { useData } from '../lib/hooks';
import type { Song, ScheduleItem } from '../lib/types';
import { confirmDialog, alertDialog } from '../components/Modal';
import { Highlight } from '../components/Highlight';
import VirtualTable from '../components/VirtualTable';

/** Title with the typed filter marked: a literal match, or failing that a phonetic (Latin for Cyrillic) one. */
function FilterMark({ text, filter }: { text: string; filter: string }) {
  const esc = filter.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/\s+/g, '\\W*');
  let range: [number, number] | null = null;
  try {
    const m = new RegExp(esc, 'i').exec(text);
    if (m && m[0]) range = [m.index, m.index + m[0].length];
  } catch { /* fall through to phonetic */ }
  if (!range) range = phoneticRange(text, filter);
  if (!range) return <>{text}</>;
  return <>{text.slice(0, range[0])}<mark>{text.slice(range[0], range[1])}</mark>{text.slice(range[1])}</>;
}

/** Songs tab: songbook filter, song table, preview with stanzas. */
export default function SongsPanel({ active }: { active: boolean }) {
  const { songbooks, settings } = useStore();
  const ui = useUI();
  const [songs, reloadSongs] = useData<Song[]>('songs', () => api.songs());
  const [songbook, setSongbook] = useState<number>(0); // 0 = all
  // the default songbook is pre-selected once, on a fresh start; afterwards the user's choice is kept
  const defaultApplied = useRef(false);
  const defaultTarget = useRef(0); // the default songbook until the state holds it (the first focus waits for that)
  useEffect(() => {
    if (defaultApplied.current || !songbooks.length) return;
    defaultApplied.current = true;
    const d = settings.app.defaultSongbook || 0;
    if (d && songbooks.some((b) => b.id === d)) {
      defaultTarget.current = d;
      setSongbook(d);
    }
  }, [songbooks, settings]);
  const [number, setNumber] = useState('');
  const [filter, setFilter] = useState('');
  const [searchType, setSearchType] = useState(0);
  const [searchIds, setSearchIds] = useState<number[] | null>(null);
  const [highlight, setHighlight] = useState('');
  const [selectedId, setSelectedId] = useState<number>(0);
  const [preview, setPreview] = useState<Song | null>(null);
  const [stanzas, setStanzas] = useState<string[]>([]);
  const [stanzaIdx, setStanzaIdx] = useState(0);
  const [sort, setSort] = useState<{ col: 'number' | 'title' | 'songbookName'; dir: 1 | -1 }>({ col: 'number', dir: 1 });
  const fromSchedule = useRef(false);
  const searchInput = useRef<HTMLInputElement>(null);
  const numberInput = useRef<HTMLInputElement>(null);
  // coming to this tab (or starting on it) puts the cursor in the Number box; without a songbook that box is
  // disabled, so the search box gets it. The default songbook arrives a moment after a fresh start.
  const focusedOnShow = useRef(false);
  useEffect(() => {
    if (!active) { focusedOnShow.current = false; return; }
    if (focusedOnShow.current) return;
    if (!songbooks.length && !songbook) return; // still loading: the default songbook may come
    if (defaultTarget.current && songbook === defaultTarget.current) defaultTarget.current = 0;
    if (defaultTarget.current) return; // the default songbook is applied but not rendered yet: wait for it
    focusedOnShow.current = true;
    (songbook ? numberInput : searchInput).current?.focus();
  }, [active, songbook, songbooks]);
  const songTable = useRef<HTMLDivElement | null>(null);
  const stanzaList = useRef<HTMLDivElement>(null);


  const list = useMemo(() => {
    let l = songs || [];
    if (searchIds) {
      const set = new Set(searchIds);
      l = l.filter((s) => set.has(s.id));
    }
    if (songbook) l = l.filter((s) => s.songbookId === songbook);
    const f = filter.trim();
    if (f && !searchIds) {
      const esc = f.replace(/[.*+?^${}()|[\]\\]/g, '\\$&').replace(/\s+/g, '\\W*');
      const re = new RegExp(esc, 'i');
      l = l.filter((s) => re.test(String(s.number)) || re.test(s.title) || phoneticContains(s.title, f));
    }
    const dir = sort.dir;
    l = [...l].sort((a, b) => {
      const av = a[sort.col];
      const bv = b[sort.col];
      if (typeof av === 'number' && typeof bv === 'number') return (av - bv) * dir;
      return String(av).localeCompare(String(bv)) * dir;
    });
    return l;
  }, [songs, songbook, filter, searchIds, sort]);

  // auto sort: numeric filter sorts by number, text sorts by title
  useEffect(() => {
    if (filter && !searchIds) setSort({ col: /^\d+$/.test(filter.trim()) ? 'number' : 'title', dir: 1 });
  }, [filter, searchIds]);

  const loadPreview = async (s: Song | null) => {
    if (!s) {
      setPreview(null);
      setStanzas([]);
      return;
    }
    const full = s.text !== undefined ? s : await api.song(s.id);
    setPreview(full);
    setStanzas((await api.stanzas(full.text || '')) ?? []);
    setStanzaIdx(0);
  };

  const select = (s: Song) => {
    setSelectedId(s.id);
    fromSchedule.current = false;
    loadPreview(s);
  };

  // select first row whenever the filtered list changes and selection is gone
  useEffect(() => {
    if (fromSchedule.current) return;
    if (list.length && !list.find((s) => s.id === selectedId)) {
      select(list[0]);
    } else if (!list.length && !fromSchedule.current) {
      setSelectedId(0);
      loadPreview(null);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [list]);

  // refresh preview after edits; select a song that was just created
  const knownIds = useRef<Set<number> | null>(null);
  useEffect(() => {
    if (songs && knownIds.current) {
      const added = songs.find((x) => !knownIds.current!.has(x.id));
      if (added) {
        knownIds.current = new Set(songs.map((x) => x.id));
        setFilter('');
        setSearchIds(null);
        select(added);
        return;
      }
    }
    if (songs) knownIds.current = new Set(songs.map((x) => x.id));
    if (preview && songs && !fromSchedule.current) {
      const s = songs.find((x) => x.id === preview.id);
      if (s) loadPreview(s);
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [songs]);

  const recordHistory = (sg: Song | null) => {
    if (sg && sg.id > 0) ui.addSongHistory({ id: sg.id, label: `${sg.number}. ${sg.title}`, songbook: sg.songbookName });
  };
  const goLive = () => {
    if (!preview) return;
    recordHistory(preview);
    api.showSong(preview.id > 0 && !fromSchedule.current ? preview.id : preview, stanzaIdx);
    focusLive();
  };
  const addToSchedule = (s?: Song) => {
    const song = s || preview;
    if (!song) return;
    const item: ScheduleItem = { type: 'song', name: `${song.number} ${song.title}`, song: song.id > 0 ? ({ id: song.id } as Song) : song };
    api.scheduleAdd(item);
  };
  const edit = () => {
    if (!preview) return alertDialog(t('No song selected'), t('No song has been selected to edit.'), t('Please select a song to edit.'));
    ui.openDialog({ kind: 'editSong', mode: 'edit', song: preview });
  };
  const copy = () => {
    if (!preview) return alertDialog(t('No song selected'), t('No song has been selected to copy'), t('Please select a song to copy'));
    ui.openDialog({ kind: 'editSong', mode: 'copy', song: preview });
  };
  const remove = async () => {
    if (!preview || preview.id <= 0) return alertDialog(t('No song selected'), t('No song has been selected to delete'), t('Please select a song to delete'));
    const r = await confirmDialog({
      title: t('Delete song?'),
      message: t('Delete song "{title}"?', { title: preview.title }),
      info: t('This action will permanently delete this song'),
      buttons: [{ label: t('Yes'), value: 'yes', primary: true }, { label: t('No'), value: 'no' }],
    });
    if (r === 'yes') {
      await api.deleteSong(preview.id);
      setSelectedId(0);
      loadPreview(null);
      reloadSongs();
    }
  };
  const print = () => {
    if (!preview) return alertDialog(t('No song selected'), t('No song has been selected to be printed.'), t('Please select a song to be printed.'));
    let s = `${preview.songbookName} ${preview.number}\n${preview.title}\n`;
    if (preview.tune) s += t('Tune: {tune}', { tune: preview.tune }) + '\n';
    if (preview.wordsBy && preview.musicBy) s += t('Words By: {w}\tMusic By: {m}', { w: preview.wordsBy, m: preview.musicBy }) + '\n\n';
    else if (preview.wordsBy) s += t('Words By: {w}', { w: preview.wordsBy }) + '\n\n';
    else if (preview.musicBy) s += t('Music By: {m}', { m: preview.musicBy }) + '\n\n';
    else s += '\n';
    s += preview.text || '';
    if (preview.notes) s += `\n\n${t('Notes:')}\n${preview.notes}`;
    ui.openDialog({ kind: 'print', title: t('Print'), text: s, fontSize: 14 });
  };

  useRegisterPanel('songs', {
    newItem: () => ui.openDialog({ kind: 'editSong', mode: 'new' }),
    edit,
    copy,
    remove,
    goLive,
    addToSchedule: () => addToSchedule(),
    print,
    focusSearch: () => { searchInput.current?.focus(); searchInput.current?.select(); },
    focusBrowser: () => stanzaList.current?.focus(),
    // the Number box needs a songbook; without one the search box takes the focus
    focusNumber: () => { const n = numberInput.current; if (n && !n.disabled) { n.focus(); n.select(); } else { searchInput.current?.focus(); searchInput.current?.select(); } },
    hasSelection: () => !!preview,
    showFromSchedule: (item) => {
      const it = item as ScheduleItem;
      if (it.song) {
        fromSchedule.current = true;
        const id = it.song.id;
        setSelectedId(id > 0 ? id : 0);
        // a song that the current songbook / filter / search hides would be selected but invisible: show everything
        if (id > 0 && !list.some((s) => s.id === id)) {
          setSongbook(0);
          setFilter('');
          setSearchIds(null);
          setHighlight('');
        }
        loadPreview(it.song);
      }
    },
  });

  const doSearch = async () => {
    const text = filter.trim();
    if (!text) {
      searchInput.current?.setAttribute('placeholder', t('Please enter search text'));
      return;
    }
    try {
      const r = await api.searchSongs(text, searchType);
      setSearchIds(r.ids);
      setHighlight(r.highlight);
    } catch (e) {
      console.error(e);
    }
  };
  const clearSearch = () => {
    setSearchIds(null);
    setHighlight('');
    setFilter('');
    searchInput.current?.setAttribute('placeholder', '');
  };

  const selectByNumber = (value: string) => {
    setNumber(value);
    const n = Number(value);
    if (!n || !songbook) return;
    const s = (songs || []).find((x) => x.number === n && x.songbookId === songbook);
    if (s) {
      if (filter) setFilter('');
      select(s);
    }
  };

  // Enter in the number box projects that song straight away
  const goLiveNumber = () => {
    const n = Number(number);
    const s = n && songbook ? (songs || []).find((x) => x.number === n && x.songbookId === songbook) : undefined;
    if (!s) return;
    recordHistory(s);
    api.showSong(s.id, 0);
    focusLive();
  };

  const columns = [
    { key: 'number', label: '#', width: 52 },
    { key: 'title', label: t('Title'), width: 0 },
    ...(songbook ? [] : [{ key: 'songbookName', label: t('Songbook'), width: 110 }]),
  ] as { key: string; label: string; width: number }[];

  const searching = searchIds !== null;

  return (
    <Splitter id="songs-main" initial={0.5} saved={settings.app.splitters} className="fill">
      <div className="col fill" style={{ gap: 4 }}>
        <div className="row">
          <select className="grow" value={songbook} onChange={(e) => setSongbook(Number(e.target.value))} title={t('Use this menu to show only songs belonging to a particular Songbook')}>
            <option value={0}>{t('All songbooks')}</option>
            {songbooks.map((sb) => (
              <option key={sb.id} value={sb.id}>{sb.name}</option>
            ))}
          </select>
          <span className="input-group">
            <input ref={numberInput} className="no-spin" type="number" min={1} placeholder={t('Number')} style={{ width: 70 }} value={number} disabled={!songbook} onChange={(e) => selectByNumber(e.target.value)} onKeyDown={(e) => { if (e.key === 'Enter') goLiveNumber(); }} title={t('Selects a song by the number in the selected Songbook')} />
            <button className="primary golive" disabled={!songbook || !number} onClick={goLiveNumber} title={t('Go Live (F5)')}><img className="icon" src="/icons/go_live.png" alt="" /></button>
          </span>
        </div>
        <div className="row">
          <span>{t('Search:')}</span>
          <input ref={searchInput} className="grow" style={{ minWidth: 0 }} type="text" value={filter} onChange={(e) => setFilter(e.target.value)} onKeyDown={(e) => {
            // Right from the end of the box moves on to the stanza list
            if (e.key === 'ArrowRight' && !e.shiftKey && e.currentTarget.selectionStart === e.currentTarget.value.length && e.currentTarget.selectionEnd === e.currentTarget.value.length) { e.preventDefault(); stanzaList.current?.focus(); }
            // Up / Down step through the (filtered) song list without leaving the box
            else if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && !e.ctrlKey && !e.metaKey && !e.altKey) {
              e.preventDefault();
              if (!list.length) return;
              const at = list.findIndex((s) => s.id === selectedId);
              const next = at < 0 ? 0 : Math.max(0, Math.min(list.length - 1, at + (e.key === 'ArrowDown' ? 1 : -1)));
              select(list[next]);
            }
            // Esc empties the box and any search results; with nothing to clear it falls through and stops the presentation
            else if (e.key === 'Escape' && (filter !== '' || searching)) { e.preventDefault(); e.stopPropagation(); clearSearch(); }
          }} />
          {searching && (
            <select title={t('Search Type:')} value={searchType} onChange={(e) => setSearchType(Number(e.target.value))}>
              <option value={0}>{t('Contains Phrase')}</option>
              <option value={1}>{t('Contains Word Phrase')}</option>
              <option value={2}>{t('Line Begins')}</option>
              <option value={3}>{t('Contains Any Word')}</option>
              <option value={4}>{t('Contains All Words')}</option>
            </select>
          )}
          <span className="btn-group">
            <button disabled={!searching && filter === ''} onClick={clearSearch}>{t('Clear')}</button>
            <button onClick={doSearch}><img className="icon" src="/icons/search.png" alt="" />{t('Search')}</button>
          </span>
        </div>
        <VirtualTable
          resizeId="songs"
          tableRef={songTable}
          // Right moves on to the stanza list
          onKeyExtra={(e) => { if (e.key === 'ArrowRight' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) { e.preventDefault(); stanzaList.current?.focus(); } }}
          className={`grow hover-rows ${active ? 'active' : ''}`}
          rows={list}
          rowKey={(s) => s.id}
          columns={columns.map((c) => ({ ...c, sort: sort.col === c.key ? sort.dir : 0 }))}
          selectedKey={selectedId}
          // typing while the list has focus starts a search: focus the box and let the character land in it
          onType={() => searchInput.current?.focus()}
          onRowDragStart={(s, e) => startScheduleDrag(e, { type: 'song', name: `${s.number} ${s.title}`, song: { id: s.id } as Song })}
          onSelect={(s) => select(s)}
          onActivate={(s) => { select(s); recordHistory(s); api.showSong(s.id, 0); focusLive(); }}
          onSort={(key) => setSort((s) => ({ col: key as typeof s.col, dir: s.col === key ? ((s.dir * -1) as 1 | -1) : 1 }))}
          render={(s, key) => (key === 'title' && highlight ? <Highlight text={s.title} pattern={highlight} /> : key === 'title' && !searching && filter.trim() ? <FilterMark text={s.title} filter={filter.trim()} /> : String((s as unknown as Record<string, unknown>)[key] ?? ''))}
        />
      </div>
      <div className="col fill" style={{ gap: 4 }}>
        <div className="row" style={{ paddingLeft: 6 }}>
          <span
            className="grow"
            draggable={!!preview}
            onDragStart={(e) => {
              if (preview) startScheduleDrag(e, { type: 'song', name: `${preview.number} ${preview.title}`, song: preview.id > 0 ? ({ id: preview.id } as Song) : preview });
            }}
            title={preview ? t('Drag to the Schedule') : undefined}
            style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minWidth: 0, cursor: preview ? 'grab' : undefined }}
          >{preview ? (preview.number > 0 ? `${preview.number}. ${preview.title}` : preview.title) : null}</span>
          <button className="primary golive" onClick={goLive} disabled={!preview} title={t('Go Live (F5)')} aria-label={t('Go Live (F5)')}><img className="icon" src="/icons/go_live.png" alt="" /></button>
        </div>
        {preview?.notes && <div className="muted" style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>{t('Notes:')}{'\n'}{preview.notes}</div>}
        <div ref={stanzaList} className="list grow hover-items" tabIndex={0} onKeyDown={(e) => {
          if (e.key === 'ArrowLeft' && !e.ctrlKey && !e.metaKey && !e.altKey && !e.shiftKey) { e.preventDefault(); songTable.current?.focus(); return; }
          if (e.key === 'ArrowDown') { setStanzaIdx((i) => Math.min(stanzas.length - 1, i + 1)); e.preventDefault(); }
          else if (e.key === 'ArrowUp') { setStanzaIdx((i) => Math.max(0, i - 1)); e.preventDefault(); }
          else if (e.key === 'Enter') goLive();
        }}>
          {stanzas.map((st, i) => (
            <div key={i} className={`item spaced nowrap ${i === stanzaIdx ? 'sel' : ''}`} onClick={() => setStanzaIdx(i)} onDoubleClick={() => { setStanzaIdx(i); recordHistory(preview); api.showSong(preview && preview.id > 0 && !fromSchedule.current ? preview.id : preview!, i); focusLive(); }}>
              <StanzaText text={st} pattern={highlight || undefined} />
            </div>
          ))}
        </div>
      </div>
    </Splitter>
  );
}
