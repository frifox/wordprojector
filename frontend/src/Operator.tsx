import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { useStore } from './lib/store';
import { api } from './lib/api';
import { t } from './i18n';
import { useShortcuts, useT, useData, cmdKey, isTyping, isMac } from './lib/hooks';
import { hotkeyMap, comboOf, comboLabel } from './lib/hotkeys';
import { UIContext, usePanelRegistry, type SongHistoryItem, type TabName, type DialogSpec, tabIcons, Icon } from './lib/ui';
import Menubar, { type MenuDef, type MenuEntry } from './components/Menubar';
import Splitter from './components/Splitter';
import Sidebar from './components/Sidebar';
import HistoryBar from './components/HistoryBar';
import type { BibleRef, UpdateStatus } from './lib/types';
import DialogHost, { } from './components/Dialogs';
import { confirmDialog } from './components/Modal';
import LivePanel from './panels/LivePanel';
import SchedulePanel from './panels/SchedulePanel';
import BiblePanel from './panels/BiblePanel';
import SongsPanel from './panels/SongsPanel';
import PicturesPanel from './panels/PicturesPanel';
import MediaPanel from './panels/MediaPanel';
import AnnouncePanel from './panels/AnnouncePanel';
import SettingsDialog from './dialogs/SettingsDialog';
import ThemeEditorHost from './dialogs/ThemeEditorHost';
import ManageDataDialog from './dialogs/ManageDataDialog';
import EditSongDialog from './dialogs/EditSongDialog';
import EditAnnouncementDialog from './dialogs/EditAnnouncementDialog';
import SlideShowEditor from './dialogs/SlideShowEditor';
import SongCounterDialog from './dialogs/SongCounterDialog';
import AboutDialog from './dialogs/AboutDialog';
import { pushUndo, undoLast, removeScheduled } from './lib/undo';
import HotkeysDialog from './dialogs/HotkeysDialog';
import PrintDialog from './dialogs/PrintDialog';
import ProgressDialog from './dialogs/ProgressDialog';


const SHOWN_TABS: TabName[] = ['bible', 'songs', 'pictures'];

export default function Operator() {
  useT();
  const store = useStore();
  const { live, schedule, settings, boot } = store;
  const [tab, setTabRaw] = useState<TabName>('songs');
  // Media and Announcements are switched off for now: their panels stay mounted (so nothing is lost and they can
  // come back by adding the ids to SHOWN_TABS) but their tabs are hidden and cannot be selected.
  const setTab = useCallback((t: TabName) => { if (SHOWN_TABS.includes(t)) setTabRaw(t); }, []);
  const [dialog, setDialog] = useState<DialogSpec | null>(null);
  const [schedSel, setSchedSel] = useState(-1);
  const registry = usePanelRegistry();
  const [bibleHistory, setBibleHistory] = useState<BibleRef[]>([]);
  const [songHistory, setSongHistory] = useState<SongHistoryItem[]>([]);
  const addBibleHistory = useCallback((r: BibleRef) => setBibleHistory((h) => [...h, r]), []);
  const addSongHistory = useCallback((s: SongHistoryItem) => setSongHistory((h) => (h.length && h[h.length - 1].id === s.id ? h : [...h, s])), []);
  const removeHistory = useCallback((tb: TabName, i: number) => {
    // taking it back puts the entry where it was
    const drop = <T,>(h: T[]) => h.filter((_, k) => k !== i);
    const back = <T,>(item: T) => (h: T[]) => [...h.slice(0, i), item, ...h.slice(i)];
    if (tb === 'bible') { const it = bibleHistory[i]; if (it) { setBibleHistory(drop); pushUndo('history-bible', () => setBibleHistory(back(it))); } }
    else if (tb === 'songs') { const it = songHistory[i]; if (it) { setSongHistory(drop); pushUndo('history-songs', () => setSongHistory(back(it))); } }
  }, [bibleHistory, songHistory]);
  const clearHistory = useCallback((tb: TabName) => {
    if (tb === 'bible') { const all = bibleHistory; if (all.length) { setBibleHistory([]); pushUndo('history-bible', () => setBibleHistory(all)); } }
    else if (tb === 'songs') { const all = songHistory; if (all.length) { setSongHistory([]); pushUndo('history-songs', () => setSongHistory(all)); } }
  }, [bibleHistory, songHistory]);

  // UI theme (light/dark)
  useEffect(() => {
    const apply = () => {
      const pref = settings.app.uiTheme || 'system';
      const dark = pref === 'dark' || (pref === 'system' && window.matchMedia('(prefers-color-scheme: dark)').matches);
      document.documentElement.setAttribute('data-theme', dark ? 'dark' : 'light');
    };
    apply();
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    mq.addEventListener('change', apply);
    return () => mq.removeEventListener('change', apply);
  }, [settings.app.uiTheme]);

  // the font of the operator window (the system font unless one is chosen)
  useEffect(() => {
    const f = settings.app.uiFont;
    const root = document.documentElement.style;
    if (f) root.setProperty('--font-ui', `"${f.replace(/"/g, '')}", -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, 'Helvetica Neue', Arial, sans-serif`);
    else root.removeProperty('--font-ui');
  }, [settings.app.uiFont]);

  // window title
  useEffect(() => {
    let title = 'Word Projector';
    if (schedule.file) {
      const name = schedule.file.split(/[\\/]/).pop()!.replace(/\.spsc$/i, '');
      title = `${name}${schedule.saved ? '' : '*'} - ${title}`;
    }
    document.title = title;
  }, [schedule.file, schedule.saved]);

  const openDialog = useCallback((d: DialogSpec) => setDialog(d), []);
  const uiValue = useMemo(
    () => ({ register: registry.register, actions: registry.actions, openDialog, setTab, tab, bibleHistory, songHistory, addBibleHistory, addSongHistory, clearHistory, removeHistory }),
    [registry.register, registry.actions, openDialog, tab, bibleHistory, songHistory, addBibleHistory, addSongHistory, clearHistory, removeHistory],
  );

  const act = (tabName: TabName = tab) => registry.actions(tabName);

  // Where the keyboard goes follows the clicks. A click on the Currently Playing panel (its header, the preview, the list) puts the
  // cursor on the selected item of its list. A click anywhere else that does not take the focus itself (the menu, the tab headers, empty
  // places of a tab) brings back what had the cursor before the panel took it: a list, or a field such as the Exact Number box.
  const tabRef = useRef(tab);
  tabRef.current = tab;
  const actRef = useRef(act);
  actRef.current = act;
  useEffect(() => {
    const away = '.live-panel, .modal, .modal-backdrop, .menubar, .menu-popup';
    let lastBrowser: HTMLElement | null = null; // what had the focus last, outside the Currently Playing panel
    let tabAtDown = tabRef.current;
    // only a list (of books, chapters, verses, songs, stanzas, slides ...) or a text field (Exact Number, Search ...) is what the
    // cursor goes back to; a button such as Go Live, a checkbox or a menu is not
    const remembered = 'input:not([type=checkbox]):not([type=radio]):not([type=button]):not([type=range]):not([type=color]), textarea, .list, .table, .thumb-list, [role="listbox"]';
    const onFocusIn = (e: FocusEvent) => {
      const el = e.target as HTMLElement | null;
      if (el && el !== document.body && el.matches?.(remembered) && !el.closest?.(away)) lastBrowser = el;
    };
    const onDown = () => { tabAtDown = tabRef.current; };
    // The list a click on the header of a column belongs to (the label above the book list, the title above the stanzas ...): the
    // nearest column that has just one list in it.
    const listOfColumn = (t: HTMLElement): HTMLElement | null => {
      for (let c = t.closest('.col') as HTMLElement | null; c && c.closest('.tabpage'); c = c.parentElement?.closest('.col') as HTMLElement | null) {
        const lists = c.querySelectorAll<HTMLElement>('.list, .table, .thumb-list');
        if (lists.length === 1) return lists[0];
        if (lists.length > 1) return null; // several columns together: not the header of one of them
      }
      return null;
    };
    const restoreBrowser = () => {
      const el = lastBrowser;
      if (el && el.isConnected && el.offsetParent !== null && !el.closest(away)) el.focus({ preventScroll: true });
      else actRef.current()?.focusBrowser?.();
    };
    const onClick = (e: MouseEvent) => {
      const t = e.target as HTMLElement | null;
      if (!t?.closest || t.closest('.modal, .modal-backdrop')) return;
      if (t.closest('input, textarea, select, [contenteditable="true"]')) return; // text fields keep what they got
      const inLive = !!t.closest('.live-panel');
      const inMenu = !!t.closest('.menubar, .menu-popup, .topbar');
      const inTabbar = !!t.closest('.tabbar');
      if (!inLive && !inMenu && !inTabbar && t.closest('button, label, a, .list, .table, .thumb-list, [class*="dd-"], .sidebar, .splitter > .gutter')) return; // controls and lists see to their focus themselves
      window.setTimeout(() => {
        if (document.querySelector('.modal-backdrop')) return; // a dialog came up
        if (inLive) window.dispatchEvent(new Event('wp-focus-live'));
        else if (inTabbar && tabRef.current !== tabAtDown) return; // another tab: it puts the cursor where it wants it
        else if (!inMenu && !inTabbar && t.closest('.tabpage') && (() => { const l = listOfColumn(t); if (l) l.focus({ preventScroll: true }); return !!l; })()) return;
        else restoreBrowser();
      }, 0);
    };
    document.addEventListener('focusin', onFocusIn);
    document.addEventListener('mousedown', onDown, true);
    document.addEventListener('click', onClick);
    return () => {
      document.removeEventListener('focusin', onFocusIn);
      document.removeEventListener('mousedown', onDown, true);
      document.removeEventListener('click', onClick);
    };
  }, []);

  // ---- schedule / file actions ----
  const saveSchedule = async (as: boolean) => {
    const r = await api.scheduleSave(as).catch((e) => {
      confirmDialog({ title: t('Error'), message: String(e.message || e), buttons: [{ label: t('OK'), value: 'ok', primary: true }] });
      return null;
    });
    if (r && r.download) {
      const { downloadURL } = await import('./lib/files');
      downloadURL(r.download);
    }
  };
  const quit = async () => {
    if (!schedule.saved && schedule.file) {
      const r = await confirmDialog({
        title: t('Schedule not saved'),
        message: t('Do you want to save current schedule?'),
        buttons: [
          { label: t('Save'), value: 'save', primary: true },
          { label: t('Discard'), value: 'discard' },
          { label: t('Cancel'), value: 'cancel' },
        ],
      });
      if (r === 'cancel') return;
      if (r === 'save') await saveSchedule(false);
    }
    // a desktop window onto another instance quits itself (through its helper) and that instance with it
    const helper = /wpquit=(\d+)/.exec(window.location.hash);
    if (helper) fetch(`http://127.0.0.1:${helper[1]}/quit`, { method: 'POST', mode: 'no-cors' }).catch(() => {});
    else if (boot.desktop) api.quit();
    else window.close();
  };

  // ---- edit actions (context sensitive) ----
  const editLabels: Record<TabName, { n?: string; e?: string; c?: string; d?: string; ni?: string; ei?: string; ci?: string; di?: string }> = {
    bible: {},
    songs: { n: t('New Song...'), e: t('Edit Song...'), c: t('Copy Song...'), d: t('Delete Song'), ni: 'song_new.png', ei: 'song_edit.png', ci: 'song_copy.png', di: 'song_delete.png' },
    pictures: {},
    media: { n: t('Add Media Files...'), d: t('Remove Media Files'), ni: 'video_add.png', di: 'video_remove.png' },
    announce: { n: t('New Announcement...'), e: t('Edit Announcement...'), c: t('Copy Announcement...'), d: t('Delete Announcement'), ni: 'announce_new.png', ei: 'announce_edit.png', ci: 'announce_copy.png', di: 'announce_delete.png' },
  };
  const el = editLabels[tab];
  const canPrint = tab === 'bible' || tab === 'songs' || tab === 'announce';

  // The On/Off button opens and closes the projection windows on the target displays. With a single screen the
  // window only exists while something is shown.
  const displaysOn = live.singleScreen ? live.showing : live.displayOn;
  const turnDisplaysOff = () => (live.singleScreen ? api.hide() : api.display(false));
  const toggleDisplays = () => {
    if (displaysOn) turnDisplaysOff();
    else if (live.singleScreen && live.type) api.show();
    else api.display(true); // also with nothing prepared: the displays show a blank slide
  };

  // ---- shortcuts ----
  const keys = hotkeyMap(settings.app.hotkeys);
  useShortcuts((e) => {
    if (dialog) return false;
    const typing = isTyping(e);
    const cmd = cmdKey(e);
    const k = e.key;
    const combo = comboOf(e);
    // a plain key (a letter, Del, ...) belongs to the focused text field while one is being typed in
    const plain = !!combo && !/^(Mod|Alt|Ctrl|Meta)\+|^F\d+$|^Escape$|^Shift\+Escape$|^Page(Up|Down)$/.test(combo);
    const is = (id: string) => !!combo && keys[id] === combo && !(typing && plain);
    // the cursor goes back to the tab's browser after the presentation stops, unless it is in the list of what is playing
    const inLive = !!(document.activeElement as HTMLElement | null)?.closest?.('.live-list, .live-thumbs');
    // Esc pauses; pressed again it turns the presentation off, like the Off button (decided by the engine)
    if (is('pause')) {
      // the second Esc stops the presentation (see Engine.Escape); the cursor then goes back to the tab's browser
      const stops = !(displaysOn && live.type && !live.cleared);
      api.escape().then(() => { if (stops && !inLive) act()?.focusBrowser?.(); });
      return true;
    }
    if (is('pauseOnly')) { api.pause(); return true; }
    // Del turns the presentation off and clears the live list, unless the focused control uses the key itself (text fields)
    // the Backspace key (the "delete" key on Mac laptops) also does it, but lists keep it: it edits the Bible filters
    const inList = !!(e.target as HTMLElement | null)?.closest?.('.list, .table');
    if (is('stop') || (keys.stop === 'Delete' && combo === 'Backspace' && !typing && !inList)) { api.stop().then(() => { if (!inLive) act()?.focusBrowser?.(); }); return true; }
    if (is('play')) {
      // With the cursor in the Currently Playing panel Play presents what is selected in its list (in multi-verse mode first a
      // pending selection). With it anywhere else (a list, a field such as Exact Number or Search, nothing) it goes live with what is
      // selected in the tab: the item of the list, the number typed, ...
      const inLive = !!(e.target as HTMLElement | null)?.closest?.('.live-panel');
      if (inLive) {
        const ev = { handled: false };
        window.dispatchEvent(new CustomEvent('wp-live-update', { detail: ev }));
        if (!ev.handled) api.show();
      } else if (act()?.goLive) act()?.goLive?.();
      else api.show();
      return true;
    }
    if (is('tabBible')) { setTab('bible'); setTimeout(() => act('bible')?.focusBook?.(), 0); return true; }
    if (is('tabSongs')) { setTab('songs'); setTimeout(() => act('songs')?.focusNumber?.(), 0); return true; }
    if (is('tabSlides')) { setTab('pictures'); return true; }
    if (is('next')) { api.next(); return true; }
    if (is('prev')) { api.prev(); return true; }
    if (is('removeItem') || (keys.removeItem === 'Mod+Delete' && combo === 'Mod+Backspace')) { act()?.remove?.(); return true; }
    if (is('help') || (keys.help === 'Mod+Shift+?' && combo === 'Mod+/')) { openDialog({ kind: 'hotkeys' }); return true; }
    // the undo key of the system takes back the last removal from the Schedule or a history, unless a text field has its own undo (on a Mac Ctrl+Z works too)
    if ((combo === 'Mod+Z' || (isMac && combo === 'Ctrl+Z')) && !typing) { if (undoLast()) return true; }
    if (is('settings')) { openDialog({ kind: 'settings' }); return true; }
    if (is('print')) { if (canPrint) act()?.print?.(); return true; }
    if (is('exit')) { quit(); return true; }
    if (!typing && (k === 'Delete' || k === 'Backspace') && !cmd) {
      const target = e.target as HTMLElement;
      if (target && target.closest && target.closest('.schedule-area')) {
        if (schedSel >= 0) removeScheduled(schedule.items, schedSel);
        return true;
      }
      return false;
    }
    if (!typing && !cmd) {
      const target = e.target as HTMLElement;
      const inList = target && target.closest && target.closest('.list, .table');
      if (!inList) {
        if (k === 'ArrowRight' || k === 'ArrowDown' || k === 'Enter') { api.next(); return true; }
        if (k === 'ArrowLeft' || k === 'ArrowUp') { api.prev(); return true; }
      }
    }
    return false;
  });

  // ---- updates ----
  const [update] = useData<UpdateStatus>('update', () => api.updateStatus());
  // a newer release exists and is not installed by itself
  const newRelease = !!update?.newer && !settings.app.autoInstallUpdates;

  // ---- menus ----
  const mi = (label: string, onClick: () => void, shortcut?: string, extra?: Partial<MenuEntry>): MenuEntry => ({ label: label.replace(/(\.{3}|…)$/, ''), onClick, shortcut: shortcut || undefined, ...extra });
  const sep: MenuEntry = { separator: true };
  const menus: MenuDef[] = [
    {
      title: t('File'),
      items: [
        mi(t('Exit'), quit, comboLabel(keys.exit), { icon: 'base-22x22-actions-application-exit.png' }),
      ],
    },
    {
      title: t('Edit'),
      items: [
        ...(el.n ? [mi(el.n, () => act()?.newItem?.(), undefined, { icon: el.ni })] : []),
        ...(el.e ? [mi(el.e, () => act()?.edit?.(), undefined, { icon: el.ei })] : []),
        ...(el.c ? [mi(el.c, () => act()?.copy?.(), undefined, { icon: el.ci })] : []),
        ...(el.d ? [mi(el.d, () => act()?.remove?.(), comboLabel(keys.removeItem), { icon: el.di })] : []),
        ...(el.n || el.e || el.c || el.d ? [sep] : []),
        mi(t('Books...'), () => openDialog({ kind: 'manage' }), undefined, { icon: 'database.png' }),
        mi(t('Themes'), () => openDialog({ kind: 'themeEditor' }), undefined, { icon: 'typography.png' }),
        mi(t('Settings...'), () => openDialog({ kind: 'settings' }), comboLabel(keys.settings), { icon: 'settings.png' }),
      ],
    },
    {
      title: t('Help'),
      // a new release waits to be installed: a mark on the menu and an item that opens Settings > Updates
      badge: newRelease ? t('New Release Available') : undefined,
      items: [
        ...(newRelease ? [mi(t('New Release Available'), () => openDialog({ kind: 'settings', page: 'updates' })), sep] : []),
        mi(t('Hotkeys'), () => openDialog({ kind: 'hotkeys' }), comboLabel(keys.help), { icon: 'hotkeys.png' }),
        mi(t('About'), () => openDialog({ kind: 'about' }), undefined, { icon: 'help.png' }),
      ],
    },
  ];

  const tabs: { id: TabName; label: string }[] = ([
    { id: 'bible', label: t('Bible') },
    { id: 'songs', label: t('Songs') },
    { id: 'pictures', label: t('Slides') },
    { id: 'media', label: t('Media') },
    { id: 'announce', label: t('Announcements (F8)') },
  ] as { id: TabName; label: string }[]).filter((tb) => SHOWN_TABS.includes(tb.id));


  return (
    <UIContext.Provider value={uiValue}>
      <div className="col fill" style={{ gap: 0 }}>
        <div className="topbar">
        <Menubar menus={menus} />
        <div className="toolbar">
          <button className={`tbtn ${displaysOn ? 'checked' : ''}`} title={displaysOn ? t('Turn displays off') : t('Turn displays on')} aria-label={displaysOn ? t('Turn displays off') : t('Turn displays on')} aria-pressed={displaysOn}
            onClick={toggleDisplays}>
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 6a7.75 7.75 0 1 0 10 0" /><path d="M12 4l0 8" /></svg>
          </button>
        </div>
        </div>
        <div className="grow" style={{ display: 'flex', minHeight: 0, padding: 4 }}>
          <Splitter id="center" initial={0.72} saved={settings.app.splitters} className="fill" min={280} maxSecond={400}>
            <div className="col fill" style={{ gap: 0 }}>
              <div className="grow" style={{ display: 'flex', minHeight: 0 }}>
                <Sidebar saved={settings.app.splitters}>
                  <div className="fill schedule-area">
                    <SchedulePanel selected={schedSel} setSelected={setSchedSel} onAdd={() => act()?.addToSchedule?.()} />
                  </div>
                </Sidebar>
                <div className="tabs grow">
                  <div className="tabbar">
                    {tabs.map((tb) => (
                      <div key={tb.id} className={`tab ${tab === tb.id ? 'active' : ''}`} onClick={() => setTab(tb.id)}>
                        <Icon name={tabIcons[tb.id]} /> {tb.label}
                      </div>
                    ))}
                  </div>
                  <div className="tabpage">
                    <div className="fill" style={{ display: tab === 'bible' ? 'flex' : 'none', flexDirection: 'column' }}><BiblePanel active={tab === 'bible'} /></div>
                    <div className="fill" style={{ display: tab === 'songs' ? 'flex' : 'none', flexDirection: 'column' }}><SongsPanel active={tab === 'songs'} /></div>
                    <div className="fill" style={{ display: tab === 'pictures' ? 'flex' : 'none', flexDirection: 'column' }}><PicturesPanel active={tab === 'pictures'} /></div>
                    <div className="fill" style={{ display: tab === 'media' ? 'flex' : 'none', flexDirection: 'column' }}><MediaPanel active={tab === 'media'} /></div>
                    <div className="fill" style={{ display: tab === 'announce' ? 'flex' : 'none', flexDirection: 'column' }}><AnnouncePanel active={tab === 'announce'} /></div>
                  </div>
                </div>
              </div>
              <HistoryBar saved={settings.app.splitters} />
            </div>
            <div className="fill" style={{ paddingLeft: 2 }}>
              <LivePanel />
            </div>
          </Splitter>
        </div>
      </div>
      {dialog?.kind === 'settings' && <SettingsDialog initialPage={dialog.page} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'themeEditor' && <ThemeEditorHost onClose={() => setDialog(null)} />}
      {dialog?.kind === 'manage' && <ManageDataDialog onClose={() => setDialog(null)} initialTab={dialog.tab} />}
      {dialog?.kind === 'editSong' && <EditSongDialog mode={dialog.mode} song={dialog.song} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'editAnnouncement' && <EditAnnouncementDialog mode={dialog.mode} announcement={dialog.announcement} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'slideshow' && <SlideShowEditor slideshow={dialog.slideshow} onClose={() => setDialog(null)} />}
      {dialog?.kind === 'counter' && <SongCounterDialog onClose={() => setDialog(null)} />}
      {dialog?.kind === 'hotkeys' && <HotkeysDialog onClose={() => setDialog(null)} />}
      {dialog?.kind === 'about' && <AboutDialog onClose={() => setDialog(null)} version={boot.version} />}
      {dialog?.kind === 'print' && <PrintDialog title={dialog.title} text={dialog.text} fontSize={dialog.fontSize} onClose={() => setDialog(null)} />}
      <ProgressDialog />
      <DialogHost />
    </UIContext.Provider>
  );
}

export { isMac };
