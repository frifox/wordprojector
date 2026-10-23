import { Fragment, memo, useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import { useRegisterPanel, useUI } from '../lib/ui';
import { useData } from '../lib/hooks';
import type { Book, ChapterVerse, SearchHit, BibleRef, ScheduleItem } from '../lib/types';
import { Highlight } from '../components/Highlight';
import { focusLive } from '../lib/focus';
import { phoneticContains, phoneticStartsWith } from '../lib/translit';

const VerseID = (b: number, c: number, v: number) => `B${String(b).padStart(3, '0')}C${String(c).padStart(3, '0')}V${String(v).padStart(3, '0')}`;

/** Bible tab: book/chapter/verse browsing, search and history. */
// One page of search results. Memoised so typing in the search box does not re-render (and re-highlight)
// thousands of rows on every keystroke.
const ResultRows = memo(function ResultRows({ results, highlight, selected, limit, onSelect, onGo }: {
  results: SearchHit[]; highlight: string; selected: number; limit: number; onSelect: (i: number) => void; onGo: (i: number) => void;
}) {
  return (
    <>
      {results.slice(0, limit).map((h, i) => (
        <div key={i} className={`results-row ${i === selected ? 'sel' : ''}`} onClick={() => onSelect(i)} onDoubleClick={() => onGo(i)}>
          <span className="cell">{h.book}</span>
          <span className="cell">{h.chapter}</span>
          <span className="cell verse-cell">
            <span className="verse-num">{h.verse}.</span>
            <span className="verse-text"><Highlight text={h.text} pattern={highlight} /></span>
          </span>
        </div>
      ))}
    </>
  );
});
const RESULT_PAGE = 100;

// Visual breaks in the (unfiltered) book list: one hairline between the testaments and plain extra space
// between the book groups. Positions follow the standard 66 book order, whatever the language.
const NEW_TESTAMENT = 40; // Matthew
const GROUP_STARTS = new Set([6, 18, 23, 28, 44, 45, 58, 66]); // History, Poetry, Major prophets, Minor prophets, Acts, Pauline letters, General letters, Revelation
function bookGap(id: number, index: number, unfiltered: boolean): React.CSSProperties | undefined {
  if (!unfiltered || index === 0) return undefined;
  if (GROUP_STARTS.has(id)) return { marginTop: 14 };
  return undefined;
}

export default function BiblePanel({ active }: { active: boolean }) {
  const { operatorBible, bibles } = useStore();
  const ui = useUI();
  const [books] = useData<Book[]>('bibles', () => (operatorBible ? api.books(operatorBible) : Promise.resolve([])), [operatorBible]);
  const [bookFilter, setBookFilter] = useState('');
  const [bookIdx, setBookIdx] = useState(0);
  const [chapter, setChapter] = useState(1);
  const [verses, setVerses] = useState<ChapterVerse[]>([]);
  const [sel, setSel] = useState<number[]>([0]);
  const [chapterText, setChapterText] = useState('');
  const [verseText, setVerseText] = useState('');
  const [searchText, setSearchText] = useState('');
  const [anyOrder, setAnyOrder] = useState(false);
  const [fullWord, setFullWord] = useState(false);
  const [results, setResults] = useState<SearchHit[] | null>(null);
  const [highlight, setHighlight] = useState('');
  const [resultSel, setResultSel] = useState(-1);
  const bookInput = useRef<HTMLInputElement>(null);
  const searchInput = useRef<HTMLInputElement>(null);
  const verseList = useRef<HTMLDivElement>(null);
  const anchor = useRef(0);
  const bookList = useRef<HTMLDivElement>(null);
  // opening the Bible tab puts the cursor on the book list: typing starts filtering the books right away
  useEffect(() => {
    if (!active) return;
    const id = window.setTimeout(() => bookList.current?.focus({ preventScroll: true }), 0); // the tab is shown first
    return () => window.clearTimeout(id);
  }, [active]);
  const chapterInput = useRef<HTMLInputElement>(null);
  const verseInput = useRef<HTMLInputElement>(null);
  const chapterList = useRef<HTMLDivElement>(null);
  const verseBuf = useRef({ text: '', timer: 0 });

  const allBooks = books || [];
  // book filter supports "1King 3 13" style quick navigation
  const parsed = useMemo(() => {
    const text = bookFilter.trim();
    let ch = 0;
    let vs = 0;
    const words = text.split(/\s+/).filter(Boolean);
    let name = text;
    if (words.length > 1) {
      const n1 = Number(words[words.length - 1]);
      if (Number.isInteger(n1)) {
        ch = n1;
        words.pop();
        if (words.length > 1) {
          const n2 = Number(words[words.length - 1]);
          if (Number.isInteger(n2)) {
            words.pop();
            ch = n2;
            vs = n1;
          }
        }
        name = words.join(' ');
      }
    }
    return { name, ch, vs };
  }, [bookFilter]);

  const filteredBooks = useMemo(() => {
    const text = parsed.name;
    if (!text) return allBooks;
    if (/^\d/.test(text)) {
      const num = text[0];
      const rest = text.slice(1).trim().toLowerCase();
      return allBooks.filter((b) => {
        const w = b.name.split(' ');
        return w[0].startsWith(num) && ((w[1] || '').toLowerCase().startsWith(rest) || phoneticStartsWith(w[1] || '', rest));
      });
    }
    const lower = text.toLowerCase();
    return allBooks.filter((b) => b.name.toLowerCase().includes(lower) || phoneticContains(b.name, text));
  }, [allBooks, parsed.name]);

  const book = filteredBooks[bookIdx] || filteredBooks[0];

  // verse box: "5" jumps to a verse, "1-5" or "1-5,10-12" filters the list to those verses
  const verseRanges = useMemo(() => {
    const txt = verseText.trim();
    if (!/[-,]/.test(txt)) return null;
    if (!/^\d+(\s*-\s*\d+)?(\s*,\s*\d+(\s*-\s*\d+)?)*$/.test(txt)) return null;
    return txt.split(',').map((part) => {
      const [a, b] = part.split('-').map((x) => Number(x.trim()));
      return [a, b === undefined ? a : b] as [number, number];
    });
  }, [verseText]);
  const filterActive = verseRanges !== null;
  const filterActiveRef = useRef(false);
  filterActiveRef.current = filterActive;
  const visible = useMemo(() => {
    const out: number[] = [];
    verses.forEach((v, i) => {
      void v;
      out.push(i); // the verse list is never filtered; typed ranges are highlighted (selected) instead
    });
    return out;
  }, [verses]);
  const visibleSet = useMemo(() => new Set(visible), [visible]);

  // chapter box: typing digits filters the chapter list to numbers starting with them
  const chapterFilter = chapterText.replace(/\D/g, '');
  const visibleChapters = useMemo(() => {
    const all = Array.from({ length: book?.chapters || 0 }, (_, i) => i + 1);
    return chapterFilter ? all.filter((n) => String(n).startsWith(chapterFilter)) : all;
  }, [book, chapterFilter]);
  const applyChapterText = (txt: string) => {
    const digits = txt.replace(/\D/g, '').slice(0, 3);
    setChapterText(digits);
    if (!digits || !book) return;
    const matches = Array.from({ length: book.chapters }, (_, i) => i + 1).filter((n) => String(n).startsWith(digits));
    if (!matches.length) return;
    const n = Number(digits);
    setChapter(matches.includes(n) ? n : matches[0]);
  };

  // when filter changes select first book and apply chapter/verse from the filter text
  useEffect(() => {
    setBookIdx(0);
    if (parsed.ch > 0) setChapter(parsed.ch);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [parsed.name, parsed.ch]);

  useEffect(() => {
    if (!book) return;
    if (chapter > book.chapters) setChapter(1);
  }, [book, chapter]);

  const load = useCallback(async () => {
    if (!operatorBible || !book) {
      setVerses([]);
      return;
    }
    const v = await api.chapter(operatorBible, book.id, chapter);
    setVerses(v);
  }, [operatorBible, book, chapter]);

  useEffect(() => {
    load();
  }, [load]);

  useEffect(() => {
    if (parsed.vs > 0 && parsed.vs <= verses.length) {
      setSel([parsed.vs - 1]);
      anchor.current = parsed.vs - 1;
    } else if (!parsed.vs) {
      setSel((s) => (s.length && s[0] < verses.length ? s : [0]));
    }
  }, [verses, parsed.vs]);

  useEffect(() => {
    const el = verseList.current?.querySelector('.item.sel') as HTMLElement | null;
    el?.scrollIntoView({ block: 'nearest' });
  }, [sel]);

  const captionFor = (s: number[]): BibleRef => {
    const ids = s.map((i) => verses[i]?.ids).filter(Boolean).join(',');
    const first = s[0];
    const last = s[s.length - 1];
    const vt = verses[first]?.text || '';
    if (first === last) {
      return { verseIds: ids, caption: `${book?.name} ${chapter}:${first + 1}`, captionLong: `${book?.name} ${chapter}:${first + 1}. ${vt}` };
    }
    const cap = `${book?.name} ${chapter}:${first + 1}-${last + 1}`;
    return { verseIds: ids, caption: cap, captionLong: `${cap}. ${vt}...` };
  };

  const goLive = (addHistory = true) => {
    if (!book || !verses.length || !sel.length) return;
    api.showBible(book.id, chapter, sel);
    focusLive();
    if (addHistory) {
      const h = captionFor(sel);
      ui.addBibleHistory(h);
    }
  };

  const addToSchedule = () => {
    if (!book || !verses.length || !sel.length) return;
    const item: ScheduleItem = { type: 'bible', name: '', bible: captionFor(sel) };
    api.scheduleAdd(item);
  };

  const selectRef = async (ref: BibleRef) => {
    if (!operatorBible) return;
    const r = await api.bibleRef(operatorBible, ref.verseIds);
    setBookFilter('');
    const idx = allBooks.findIndex((b) => b.id === r.bookId);
    if (idx >= 0) setBookIdx(idx);
    setChapter(r.chapter);
    const v = await api.chapter(operatorBible, r.bookId, r.chapter);
    setVerses(v);
    const s: number[] = [];
    v.forEach((cv, i) => {
      if (cv.num >= r.first && cv.num <= r.last) s.push(i);
    });
    setSel(s.length ? s : [0]);
  };

  // Esc in the search box clears the typed text and any shown results (when there is
  // nothing to clear Esc is left alone and hides the display)
  const onSearchKey = (e: React.KeyboardEvent) => {
    if (e.key === 'Enter') doSearch();
    else if (e.key === 'Tab' && !e.shiftKey && !e.ctrlKey && !e.metaKey && !e.altKey) {
      // skip Any Order / Full Word / Search and land on the book list (or the results)
      const target = results !== null ? resultsRef.current : bookList.current;
      if (target) {
        e.preventDefault();
        target.focus();
      }
    } else if (e.key === 'Escape' && (results !== null || searchText !== '')) {
      e.preventDefault();
      e.stopPropagation();
      setSearchText('');
      setResults(null);
      setResultSel(-1);
    }
  };

  // empties the search box and the results
  const clearSearch = () => {
    setSearchText('');
    setResults(null);
    setResultSel(-1);
    searchInput.current?.focus();
  };

  const doSearch = async () => {
    // submitting an emptied search box after a search means "Clear Search"
    if (!searchText.trim()) {
      if (results !== null) {
        setResults(null);
        setResultSel(-1);
      }
      return;
    }
    if (!operatorBible) return;
    try {
      const r = await api.searchBible(operatorBible, searchText, !anyOrder, fullWord, 0, 0, 0);
      setResults(r.hits);
      setHighlight(r.highlight);
      setResultSel(-1);
    } catch (e) {
      console.error(e);
    }
  };

  const selectResult = (i: number) => {
    const h = results?.[i];
    if (!h) return;
    setResultSel(i);
    setBookFilter('');
    const idx = allBooks.findIndex((b) => b.id === h.bookId);
    if (idx >= 0) setBookIdx(idx);
    setChapter(h.chapter);
    setTimeout(() => {
      setSel([h.verse - 1]);
      anchor.current = h.verse - 1;
    }, 50);
  };

  const resultsRef = useRef<HTMLDivElement>(null);
  // only the first rows of a long result list are rendered; more are added while scrolling
  const [shown, setShown] = useState(RESULT_PAGE);
  useEffect(() => { setShown(RESULT_PAGE); }, [results]);
  const rowHandlers = useRef({ select: (_i: number) => {}, go: (_i: number) => {} });
  const onRowSelect = useCallback((i: number) => rowHandlers.current.select(i), []);
  const onRowGo = useCallback((i: number) => rowHandlers.current.go(i), []);
  const goLiveHit = (i: number) => {
    const h = results?.[i];
    if (!h) return;
    selectResult(i);
    const id = VerseID(h.bookId, h.chapter, h.verse);
    api.showBibleRef(id);
    focusLive();
    ui.addBibleHistory({ verseIds: id, caption: `${h.book} ${h.chapter}:${h.verse}`, captionLong: `${h.book} ${h.chapter}:${h.verse}. ${h.text}` });
  };
  rowHandlers.current = { select: selectResult, go: goLiveHit };
  const onResultsKey = (e: React.KeyboardEvent) => {
    if (!results?.length) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const n = Math.max(0, Math.min(results.length - 1, resultSel + (e.key === 'ArrowDown' ? 1 : -1)));
      selectResult(n);
      if (n >= shown - 5) setShown(n + RESULT_PAGE);
      (resultsRef.current?.querySelectorAll('.results-row')[n] as HTMLElement | undefined)?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter') goLiveHit(resultSel);
  };

  const print = async () => {
    if (!operatorBible || !book) return;
    const r = await api.chapterText(operatorBible, book.id, chapter);
    const s = `${r.bible}\n${r.book} ${r.chapter}\n\n` + r.verses.map((v) => `${v.num}. ${v.text}`).join('\n');
    ui.openDialog({ kind: 'print', title: t('Print'), text: s, fontSize: 11 });
  };

  useRegisterPanel('bible', {
    goLive: () => (results !== null && resultSel >= 0 ? goLiveHit(resultSel) : goLive(true)),
    addToSchedule,
    remove: () => ui.clearHistory('bible'),
    print,
    focusBook: () => { bookInput.current?.focus(); bookInput.current?.select(); },
    focusBrowser: () => verseList.current?.focus(),
    focusSearch: () => { searchInput.current?.focus(); searchInput.current?.select(); },
    hasSelection: () => sel.length > 0 && verses.length > 0,
    showFromSchedule: (item) => {
      const it = item as ScheduleItem;
      if (it.bible) selectRef(it.bible);
    },
  });

  const dragging = useRef(false);
  const range = (a: number, b: number) => {
    const lo = Math.min(a, b);
    const hi = Math.max(a, b);
    const out: number[] = [];
    for (let k = lo; k <= hi; k++) if (visibleSet.has(k)) out.push(k);
    return out;
  };
  // press selects (shift extends), dragging across verses extends the range
  const onVerseDown = (i: number, e: React.MouseEvent) => {
    if (e.button !== 0) return;
    if (e.shiftKey) setSel(range(anchor.current, i));
    else {
      anchor.current = i;
      setSel([i]);
    }
    dragging.current = true;
    const up = () => {
      dragging.current = false;
      window.removeEventListener('mouseup', up);
    };
    window.addEventListener('mouseup', up);
  };
  const onVerseEnter = (i: number) => {
    if (dragging.current) setSel(range(anchor.current, i));
  };

  const applyVerseText = (v: string) => {
    setVerseText(v);
    const n = Number(v);
    const txt = v.trim();
    if (/[-,]/.test(txt) && /^\d+(\s*-\s*\d+)?(\s*,\s*\d+(\s*-\s*\d+)?)*$/.test(txt)) {
      const rs = txt.split(',').map((part) => {
        const [a, b] = part.split('-').map((x) => Number(x.trim()));
        return [a, b === undefined ? a : b] as [number, number];
      });
      const idx: number[] = [];
      verses.forEach((vv, i) => {
        if (rs.some(([a, b]) => vv.num >= Math.min(a, b) && vv.num <= Math.max(a, b))) idx.push(i);
      });
      if (idx.length) {
        setSel(idx);
        anchor.current = idx[0];
      }
      return;
    }
    if (v.trim() !== '' && n >= 1 && n <= verses.length) {
      setSel([n - 1]);
      anchor.current = n - 1;
    }
  };
  // typing in the verse list: digits jump to a verse, "1-5" / "1-5,10" filter the list
  const typeVerse = (e: React.KeyboardEvent): boolean => {
    if (e.ctrlKey || e.metaKey || e.altKey) return false;
    const buf = verseBuf.current;
    const arm = () => {
      window.clearTimeout(buf.timer);
      buf.timer = window.setTimeout(() => { buf.text = ''; }, 1500);
    };
    if (/^\d$/.test(e.key)) {
      let next = buf.text + e.key;
      if (!/[-,]/.test(next) && Number(next) > verses.length) next = e.key;
      buf.text = next;
      arm();
      applyVerseText(next);
      return true;
    }
    if ((e.key === '-' || e.key === ',') && buf.text && /\d$/.test(buf.text)) {
      buf.text += e.key;
      arm();
      setVerseText(buf.text);
      return true;
    }
    if (e.key === 'Backspace') {
      buf.text = buf.text.slice(0, -1);
      arm();
      applyVerseText(buf.text);
      return true;
    }
    return false;
  };

  const onVerseKey = (e: React.KeyboardEvent) => {
    if (paneNav(2, e)) return;
    if (typeVerse(e)) {
      e.preventDefault();
      return;
    }
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const pos = visible.indexOf(e.key === 'ArrowDown' ? sel[sel.length - 1] : sel[0]);
      const np = e.key === 'ArrowDown' ? Math.min(visible.length - 1, pos + 1) : Math.max(0, pos - 1);
      const cur = visible[np < 0 ? 0 : np];
      if (cur === undefined) return;
      if (e.shiftKey) setSel(range(anchor.current, cur));
      else {
        anchor.current = cur;
        setSel([cur]);
      }
    } else if (e.key === 'Enter') {
      goLive(true);
    }
  };

  // Tab / Shift+Tab inside a column input jumps to the neighbouring column's input
  const onInputTab = (idx: number) => (e: React.KeyboardEvent) => {
    if (e.key !== 'Tab' || e.ctrlKey || e.metaKey || e.altKey) return;
    const inputs = [bookInput, chapterInput, verseInput];
    const next = idx + (e.shiftKey ? -1 : 1);
    if (next < 0) return; // Shift+Tab from the book box goes back to the search controls
    e.preventDefault();
    const el = inputs[next >= inputs.length ? 0 : next].current;
    el?.focus();
    el?.select();
  };

  // Esc in any of the three columns clears all three search boxes; once they are
  // already blank Esc is left alone (it hides the display)
  const onColumnEsc = (e: React.KeyboardEvent) => {
    if (e.key !== 'Escape' || e.ctrlKey || e.metaKey || e.altKey) return;
    if (bookFilter === '' && chapterText === '' && verseText === '') return;
    e.preventDefault();
    e.stopPropagation();
    clearFilters();
  };
  const clearFilters = () => {
    window.clearTimeout(verseBuf.current.timer);
    verseBuf.current.text = '';
    setBookFilter('');
    setChapterText('');
    setVerseText('');
  };
  const filtering = bookFilter !== '' || chapterText !== '' || verseText !== '';

  // typing in the book list means "search": the characters go to the book filter
  const onBookKey = (e: React.KeyboardEvent) => {
    if (paneNav(0, e)) return;
    if (e.ctrlKey || e.metaKey || e.altKey) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      setBookIdx((i) => Math.max(0, Math.min(filteredBooks.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1))));
    } else if (e.key === 'Enter') {
      goLive(true);
    } else if (e.key === 'Backspace') {
      e.preventDefault();
      setBookFilter((f) => f.slice(0, -1));
      bookInput.current?.focus();
    } else if (e.key.length === 1) {
      e.preventDefault();
      setBookFilter((f) => f + e.key);
      bookInput.current?.focus();
    }
  };

  // chapter list: arrows move; typed digits go into the chapter box and filter the list
  // Left / Right move between the Book, Chapter and Verse panes (in a text box only from its very end / start)
  const paneNav = (idx: number, e: React.KeyboardEvent, fromInput = false): boolean => {
    if (e.ctrlKey || e.metaKey || e.altKey || e.shiftKey) return false;
    const lists = [bookList, chapterList, verseList];
    const dir = e.key === 'ArrowRight' ? 1 : e.key === 'ArrowLeft' ? -1 : 0;
    if (!dir || idx + dir < 0 || idx + dir > 2) return false;
    if (fromInput) {
      const el = e.currentTarget as HTMLInputElement;
      if (el.selectionStart !== el.selectionEnd || el.selectionStart !== (dir > 0 ? el.value.length : 0)) return false;
    }
    e.preventDefault();
    lists[idx + dir].current?.focus();
    return true;
  };
  const onChapterKey = (e: React.KeyboardEvent) => {
    if (paneNav(1, e)) return;
    if (e.ctrlKey || e.metaKey || e.altKey || !book) return;
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const pos = visibleChapters.indexOf(chapter);
      const np = Math.max(0, Math.min(visibleChapters.length - 1, (pos < 0 ? 0 : pos) + (e.key === 'ArrowDown' ? 1 : -1)));
      if (visibleChapters[np] !== undefined) setChapter(visibleChapters[np]);
    } else if (e.key === 'Enter') {
      goLive(true);
    } else if (/^\d$/.test(e.key)) {
      e.preventDefault();
      applyChapterText(chapterText.length >= 3 ? e.key : chapterText + e.key);
    } else if (e.key === 'Backspace') {
      e.preventDefault();
      applyChapterText(chapterText.slice(0, -1));
    }
  };
  useEffect(() => {
    (chapterList.current?.querySelector('.item.sel') as HTMLElement | null)?.scrollIntoView({ block: 'nearest' });
  }, [chapter, book]);
  useEffect(() => {
    (bookList.current?.querySelector('.item.sel') as HTMLElement | null)?.scrollIntoView({ block: 'nearest' });
  }, [bookIdx, filteredBooks]);

  const noBible = !operatorBible || bibles.length === 0;

  return (
    <>
      <div className="col fill" style={{ gap: 4 }}>
        {noBible && (
          <div className="group" style={{ background: 'var(--panel)' }}>
            {t('No Bible installed. Use Edit → Books to download or import a Bible.')}{' '}
            <button className="small" onClick={() => ui.openDialog({ kind: 'manage', tab: 'bibles' })}>{t('Books...')}</button>
          </div>
        )}
        <div className="row">
          <span>{t('Search:')}</span>
          <input ref={searchInput} type="text" className="grow" value={searchText} onChange={(e) => setSearchText(e.target.value)} onKeyDown={onSearchKey} />
          <label className="chk" title={t('Match the words anywhere in the verse, in any order (default: the exact phrase)')}>
            <input type="checkbox" checked={anyOrder} onChange={(e) => setAnyOrder(e.target.checked)} />
            {t('Any Order')}
          </label>
          <label className="chk" title={t('Do not match partial words')}>
            <input type="checkbox" checked={fullWord} onChange={(e) => setFullWord(e.target.checked)} />
            {t('Full Word')}
          </label>
          <button disabled={searchText === '' && results === null} onClick={clearSearch} title={t('Clear the search text and the results (Esc)')}>{t('Clear')}</button>
          <button onClick={doSearch}><img className="icon" src="/icons/search.png" alt="" />{t('Search')}</button>
        </div>
        {results !== null ? (
        <div className="col grow" style={{ minHeight: 0, gap: 4 }}>
          <div className="row">
            <b>{t('Results:')}</b>
            <span className="muted">{results.length ? t('{n} results', { n: results.length }) : t('No search results.')}</span>
            <span className="grow" />
            <button className="primary golive" onClick={() => goLiveHit(resultSel)} disabled={resultSel < 0} title={t('Go Live (F5)')} aria-label={t('Go Live (F5)')}>
              <img className="icon" src="/icons/go_live.png" alt="" />
            </button>
          </div>
          <div className="results-table list grow active" tabIndex={0} ref={resultsRef} onKeyDown={onResultsKey} onScroll={(e) => { const el = e.currentTarget; if (shown < results.length && el.scrollHeight - el.scrollTop - el.clientHeight < 600) setShown((s) => s + RESULT_PAGE); }}>
            <div className="results-head">
              <span>{t('Book:').replace(':', '')}</span>
              <span title={t('Chapter:').replace(':', '')}>{t('Ch.')}</span>
              <span>{t('Verse:').replace(':', '')}</span>
            </div>
            <ResultRows results={results} highlight={highlight} selected={resultSel} limit={shown} onSelect={onRowSelect} onGo={onRowGo} />
          </div>
        </div>
        ) : (
        <div className="row grow" style={{ alignItems: 'stretch', minHeight: 0 }} onKeyDown={onColumnEsc}>
          <div className="col" style={{ flex: '0 0 170px', gap: 3 }}>
            <span className="col-label">{t('Book:')}</span>
            <input ref={bookInput} className="hdr-ctl" tabIndex={-1} type="text" value={bookFilter} onChange={(e) => setBookFilter(e.target.value)} onKeyDown={(e) => {
              onInputTab(0)(e);
              if (paneNav(0, e, true)) return;
              if (e.key === 'Enter') goLive(true);
              // Up / Down step through the (filtered) books without leaving the box
              else if ((e.key === 'ArrowDown' || e.key === 'ArrowUp') && !e.ctrlKey && !e.metaKey && !e.altKey) {
                e.preventDefault();
                setBookIdx((i) => Math.max(0, Math.min(filteredBooks.length - 1, i + (e.key === 'ArrowDown' ? 1 : -1))));
              }
            }} />
            <div ref={bookList} className="list grow hover-items" tabIndex={0} onKeyDown={onBookKey}>
              {filteredBooks.map((b, i) => (
                <Fragment key={b.id}>
                  {/* the line between the Old and the New Testament is a row of its own, so a selected Matthew does not take the space above it */}
                  {b.id === NEW_TESTAMENT && bookFilter === '' && i > 0 && <div style={{ height: 1, margin: '14px 0 16px', background: 'var(--border)' }} />}
                  <div className={`item ${i === bookIdx ? 'sel' : ''}`} style={bookGap(b.id, i, bookFilter === '')} onClick={() => setBookIdx(i)}>{b.name}</div>
                </Fragment>
              ))}
            </div>
          </div>
          <div className="col" style={{ flex: '0 0 56px', minWidth: 0, gap: 3 }}>
            <span className="col-label">{t('Chapter:')}</span>
            <input ref={chapterInput} onKeyDown={(e) => { onInputTab(1)(e); paneNav(1, e, true); }} className="hdr-ctl" type="text" tabIndex={-1} maxLength={3} style={{ padding: '3px 5px', textAlign: 'left', width: '100%', minWidth: 0 }} value={chapterText} onChange={(e) => applyChapterText(e.target.value)} />
            <div ref={chapterList} className="list grow hover-items" tabIndex={0} onKeyDown={onChapterKey}>
              {visibleChapters.map((n) => (
                <div key={n} className={`item ${n === chapter ? 'sel' : ''}`} onClick={() => setChapter(n)}>{n}</div>
              ))}
            </div>
          </div>
          <div className="col grow" style={{ gap: 3, position: 'relative' }}>
            <span className="col-label">{t('Verse:')}</span>
            <div className="row">
              <input ref={verseInput} className="hdr-ctl" tabIndex={-1} type="text" style={{ width: 60 }} value={verseText} onChange={(e) => applyVerseText(e.target.value)} onKeyDown={(e) => { onInputTab(2)(e); if (!paneNav(2, e, true) && e.key === 'Enter') goLive(true); }} />
            </div>
            <div style={{ position: 'absolute', top: 0, right: 0, display: 'flex', gap: 6 }}>
              {filtering && (
                <button className="hdr-ctl" tabIndex={-1} onClick={clearFilters} title={t('Clear the book, chapter and verse filters (Esc)')}>
                  {t('Clear Filter')}
                </button>
              )}
              <button className="primary golive hdr-ctl" tabIndex={-1} onClick={() => goLive(true)} disabled={noBible || !verses.length} title={`${t('Quickly display the selected Bible verse on the screen')} (F5)`} aria-label={t('Go Live (F5)')}>
                <img className="icon" src="/icons/go_live.png" alt="" />
              </button>
            </div>
            <div ref={verseList} className="list grow hover-items" tabIndex={0} onKeyDown={onVerseKey}>
              {visible.map((i) => {
                const v = verses[i];
                return (
                <div key={i} className={`item verse ${sel.includes(i) ? 'sel' : ''}`} style={{ padding: '3px 6px', borderBottom: '1px solid var(--border-light)' }} onMouseDown={(e) => onVerseDown(i, e)} onMouseEnter={() => onVerseEnter(i)} onDoubleClick={() => goLive(true)}>
                  <span className="verse-num">{v.num}.</span>
                  <span className="verse-text">{v.text}</span>
                </div>
                );
              })}
            </div>
          </div>
        </div>
        )}
      </div>
    </>
  );
}
