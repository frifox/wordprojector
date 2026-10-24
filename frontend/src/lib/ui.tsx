import React, { createContext, useContext, useRef } from 'react';
import type { Announcement, Song, SlideShowFull, BibleRef } from './types';

export type TabName = 'bible' | 'songs' | 'pictures' | 'media' | 'announce';

/** Actions a tab panel exposes to the toolbar/menus. */
export interface PanelActions {
  newItem?: () => void;
  edit?: () => void;
  copy?: () => void;
  remove?: () => void;
  goLive?: () => void;
  addToSchedule?: () => void;
  print?: () => void;
  focusSearch?: () => void;
  focusNumber?: () => void;
  focusBook?: () => void;
  /** puts the keyboard focus on the tab's browser list (the stanzas, the verses, the photos) */
  focusBrowser?: () => void;
  hasSelection?: () => boolean;
  /** called when a schedule item of this panel's type is selected (preview) */
  showFromSchedule?: (item: unknown) => void;
}

export type DialogSpec =
  | { kind: 'settings'; page?: 'updates' }
  | { kind: 'themeEditor' }
  | { kind: 'manage'; tab?: 'bibles' | 'songbooks' }
  | { kind: 'editSong'; mode: 'new' | 'edit' | 'copy'; song?: Song }
  | { kind: 'editAnnouncement'; mode: 'new' | 'edit' | 'copy'; announcement?: Announcement }
  | { kind: 'slideshow'; slideshow?: SlideShowFull }
  | { kind: 'counter' }
  | { kind: 'about' }
  | { kind: 'print'; title: string; text: string; fontSize?: number };

export interface UIContextValue {
  register: (tab: TabName, actions: PanelActions) => void;
  actions: (tab: TabName) => PanelActions | undefined;
  openDialog: (d: DialogSpec) => void;
  setTab: (t: TabName) => void;
  tab: TabName;
  bibleHistory: BibleRef[];
  songHistory: SongHistoryItem[];
  addBibleHistory: (r: BibleRef) => void;
  addSongHistory: (s: SongHistoryItem) => void;
  clearHistory: (tab: TabName) => void;
  /** drops one item of a tab's history */
  removeHistory: (tab: TabName, index: number) => void;
}

export interface SongHistoryItem {
  id: number;
  label: string; // number + title
  songbook: string;
}

/** Tabs that keep a history of what was shown. */
export const historyTabs: TabName[] = ['bible', 'songs'];

export const UIContext = createContext<UIContextValue | null>(null);

export function useUI(): UIContextValue {
  const v = useContext(UIContext);
  if (!v) throw new Error('no ui context');
  return v;
}

export function usePanelRegistry() {
  const ref = useRef<Partial<Record<TabName, PanelActions>>>({});
  const register = (tab: TabName, actions: PanelActions) => {
    ref.current[tab] = actions;
  };
  const actions = (tab: TabName) => ref.current[tab];
  return { register, actions };
}

/** Hook used by panels to publish their actions. */
export function useRegisterPanel(tab: TabName, actions: PanelActions) {
  const ui = useUI();
  ui.register(tab, actions);
}

export const tabIcons: Record<TabName, string> = {
  bible: 'book.png',
  songs: 'song_tab.png',
  pictures: 'photo.png',
  media: 'video.png',
  announce: 'announce.png',
};

/** The play triangle inside the blue Go Live buttons. */
export function GoLiveIcon() {
  return (
    <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">
      <path d="M7 4v16l13 -8z" />
    </svg>
  );
}

export function Icon({ name, size = 16, style }: { name: string; size?: number; style?: React.CSSProperties }) {
  return <img src={`/icons/${name}`} alt="" width={size} height={size} style={{ verticalAlign: 'middle', ...style }} draggable={false} />;
}
