import { useCallback, useEffect, useMemo, useState } from 'react';
import { useStore } from './lib/store';
import { api } from './lib/api';
import { t } from './i18n';
import { useShortcuts, useT, useData, cmdKey, isTyping, shortcutLabel, isMac } from './lib/hooks';
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
  const clearHistory = useCallback((tb: TabName) => {
    if (tb === 'bible') setBibleHistory([]);
    else if (tb === 'songs') setSongHistory([]);
  }, []);

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
    () => ({ register: registry.register, actions: registry.actions, openDialog, setTab, tab, bibleHistory, songHistory, addBibleHistory, addSongHistory, clearHistory }),
    [registry.register, registry.actions, openDialog, tab, bibleHistory, songHistory, addBibleHistory, addSongHistory, clearHistory],
  );

  const act = (tabName: TabName = tab) => registry.actions(tabName);

  // ---- schedule / file actions ----
  const askSave = async (question: string): Promise<boolean> => {
    if (schedule.saved || !schedule.file) return true;
    const r = await confirmDialog({
      title: t('Save Schedule?'),
      message: question,
      buttons: [
        { label: t('Yes'), value: 'yes', primary: true },
        { label: t('No'), value: 'no' },
        { label: t('Cancel'), value: 'cancel' },
      ],
    });
    if (r === 'cancel') return false;
    if (r === 'yes') await saveSchedule(false);
    return true;
  };
  const newSchedule = async () => {
    if (!(await askSave(t('Do you want to save current schedule before creating a new schedule?')))) return;
    await api.scheduleNew();
  };
  const openSchedule = async () => {
    if (!(await askSave(t('Do you want to save current schedule before opening a new schedule?')))) return;
    if (boot.desktop) {
      await api.scheduleOpen().catch((e) => confirmDialog({ title: t('Error'), message: String(e.message || e), buttons: [{ label: t('OK'), value: 'ok', primary: true }] }));
    } else {
      const { pickFiles } = await import('./lib/files');
      const paths = await pickFiles(false, t('Open SoftProjector schedule:'), [{ name: t('SoftProjector schedule file (*.spsc)'), pattern: '*.spsc' }], false);
      if (paths[0]) await api.scheduleOpen(paths[0]).catch((e) => confirmDialog({ title: t('Error'), message: String(e.message || e), buttons: [{ label: t('OK'), value: 'ok', primary: true }] }));
    }
  };
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
  const printSchedule = () => {
    const name = schedule.file ? schedule.file.split(/[\\/]/).pop()!.replace(/\.spsc$/i, '') : '';
    let s = name ? `${t('SoftProject Schedule: ')}${name}\n\n` : '';
    s += schedule.items.map((it) => `${it.type}: ${it.name}`).join('\n');
    openDialog({ kind: 'print', title: t('Print Schedule'), text: s, fontSize: 11 });
  };

  // ---- edit actions (context sensitive) ----
  const editLabels: Record<TabName, { n?: string; e?: string; c?: string; d?: string; ni?: string; ei?: string; ci?: string; di?: string }> = {
    bible: { d: t('&Clear Bible History List').replace('&', ''), di: 'bibleHistoryDelete.png' },
    songs: { n: t('New Song...'), e: t('Edit Song...'), c: t('Copy Song...'), d: t('Delete Song'), ni: 'song_new.png', ei: 'song_edit.png', ci: 'song_copy.png', di: 'song_delete.png' },
    pictures: { n: t('New Slide Show...'), e: t('Edit Slide Show...'), d: t('Delete Slide Show'), ni: 'slideshow_new.png', ei: 'slideshow_edit.png', di: 'slideshow_delete.png' },
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
    else if (live.singleScreen) api.show();
    else api.display(true);
  };

  // ---- shortcuts ----
  useShortcuts((e) => {
    if (dialog) return false;
    const typing = isTyping(e);
    const cmd = cmdKey(e);
    const k = e.key;
    if (k === 'F4') { api.show(); return true; }
    // Esc pauses; pressed again it turns the presentation off and clears the live list (decided by the engine)
    if (k === 'Escape' && !e.shiftKey) {
      // the second Esc stops the presentation (see Engine.Escape); the cursor then goes back to the tab's browser
      const stops = !(displaysOn && live.type && !live.cleared);
      api.escape().then(() => { if (stops) act()?.focusBrowser?.(); });
      return true;
    }
    if (k === 'Escape' && e.shiftKey) { api.pause(); return true; }
    // Del turns the presentation off and clears the live list, unless the focused control uses the key itself (text fields)
    if (!typing && !cmd && !e.altKey && !e.shiftKey) {
      const inList = !!(e.target as HTMLElement | null)?.closest?.('.list, .table');
      // the Backspace key (the "delete" key on Mac laptops) also edits the Bible filters, so lists keep it
      if (k === 'Delete' || (k === 'Backspace' && !inList)) { api.stop().then(() => act()?.focusBrowser?.()); return true; }
    }
    if (k === 'F5') {
      // in multi-verse mode F5 first applies a pending selection from the live list
      const ev = { handled: false };
      window.dispatchEvent(new CustomEvent('wp-live-update', { detail: ev }));
      if (ev.handled) return true;
      // F5 is Play; with a verse or song focused in a browser list it pushes that item instead
      const inBrowserList = !!(e.target as HTMLElement | null)?.closest?.('.tabpage .list, .tabpage .table');
      if (inBrowserList && act()?.goLive) act()?.goLive?.();
      else api.show();
      return true;
    }
    if (k === 'F6') { setTab('bible'); setTimeout(() => act('bible')?.focusBook?.(), 0); return true; }
    if (k === 'F7') { setTab('songs'); setTimeout(() => act('songs')?.focusNumber?.(), 0); return true; }
    if (k === 'F8') { setTab('pictures'); return true; }
    if (k === 'F9') { setTab('bible'); setTimeout(() => act('bible')?.focusSearch?.(), 0); return true; }
    if (k === 'F2') { act()?.addToSchedule?.(); return true; }
    if (k === 'PageDown') { api.next(); return true; }
    if (k === 'PageUp') { api.prev(); return true; }
    if (!typing && (k === 'Delete' || k === 'Backspace') && !cmd) {
      const target = e.target as HTMLElement;
      if (target && target.closest && target.closest('.schedule-area')) {
        if (schedSel >= 0) api.scheduleRemove(schedSel);
        return true;
      }
      return false;
    }
    if (cmd && !e.shiftKey) {
      switch (k.toLowerCase()) {
        case 'n': act()?.newItem?.(); return true;
        case 'e': act()?.edit?.(); return true;
        case 'c':
          if (typing || window.getSelection()?.toString()) return false;
          act()?.copy?.(); return true;
        case 'delete': case 'backspace': act()?.remove?.(); return true;
        case 't': case ',': openDialog({ kind: 'settings' }); return true;
        case 'm': openDialog({ kind: 'manage' }); return true;
        case 's': saveSchedule(false); return true;
        case 'o': openSchedule(); return true;
        case 'p': if (canPrint) act()?.print?.(); return true;
        case 'q': quit(); return true;
      }
    }
    if (cmd && e.shiftKey) {
      switch (k.toLowerCase()) {
        case 'n': newSchedule(); return true;
        case 'p': printSchedule(); return true;
      }
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
  const mi = (label: string, onClick: () => void, shortcut?: string, extra?: Partial<MenuEntry>): MenuEntry => ({ label, onClick, shortcut: shortcut ? shortcutLabel(shortcut) : undefined, ...extra });
  const sep: MenuEntry = { separator: true };
  const menus: MenuDef[] = [
    {
      title: t('File'),
      items: [
        mi(t('Exit'), quit, 'Ctrl+Q', { icon: 'base-22x22-actions-application-exit.png' }),
      ],
    },
    {
      title: t('Edit'),
      items: [
        ...(el.n ? [mi(el.n, () => act()?.newItem?.(), 'Ctrl+N', { icon: el.ni })] : []),
        ...(el.e ? [mi(el.e, () => act()?.edit?.(), 'Ctrl+E', { icon: el.ei })] : []),
        ...(el.c ? [mi(el.c, () => act()?.copy?.(), 'Ctrl+C', { icon: el.ci })] : []),
        ...(el.d ? [mi(el.d, () => act()?.remove?.(), 'Ctrl+Del', { icon: el.di })] : []),
        sep,
        mi(t('Books...'), () => openDialog({ kind: 'manage' }), 'Ctrl+M', { icon: 'database.png' }),
        mi(t('Theme Editor...'), () => openDialog({ kind: 'themeEditor' }), undefined, { icon: 'typography.png' }),
        mi(t('Settings...'), () => openDialog({ kind: 'settings' }), 'Ctrl+,', { icon: 'settings.png' }),
      ],
    },
    {
      title: t('Help'),
      // a new release waits to be installed: a mark on the menu and an item that opens Settings > Updates
      badge: newRelease ? t('New Release Available') : undefined,
      items: [
        ...(newRelease ? [mi(t('New Release Available'), () => openDialog({ kind: 'settings', page: 'updates' })), sep] : []),
        mi(t('About'), () => openDialog({ kind: 'about' })),
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
          <button className={`tbtn ${displaysOn ? 'checked' : ''}`} title={displaysOn ? `${t('Turn displays off')} (Esc)` : t('Turn displays on')} aria-label={displaysOn ? t('Turn displays off') : t('Turn displays on')} aria-pressed={displaysOn}
            onClick={toggleDisplays} disabled={live.singleScreen && !displaysOn && !live.type}>
            <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M7 6a7.75 7.75 0 1 0 10 0" /><path d="M12 4l0 8" /></svg>
          </button>
        </div>
        </div>
        <div className="grow" style={{ display: 'flex', minHeight: 0, padding: 4 }}>
          <Splitter id="center" initial={0.72} saved={settings.app.splitters} className="fill" fixedSecond min={280}>
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
      {dialog?.kind === 'about' && <AboutDialog onClose={() => setDialog(null)} version={boot.version} />}
      {dialog?.kind === 'print' && <PrintDialog title={dialog.title} text={dialog.text} fontSize={dialog.fontSize} onClose={() => setDialog(null)} />}
      <ProgressDialog />
      <DialogHost />
    </UIContext.Provider>
  );
}

export { isMac };
