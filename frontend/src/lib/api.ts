import { trackPicker } from './picker';
import type {
  Announcement, BibleInfo, Book, Bootstrap, ChapterVerse, CountRow, MediaItem, Module, ScheduleItem, ScheduleState,
  RenderSlide, ThemeImage, UpdateStatus, SearchHit, Settings, Slide, SlideShow, SlideShowFull, Song, Songbook, Theme, ThemeInfo, Screen,
} from './types';

export class ApiError extends Error {
  status: number;
  constructor(status: number, message: string) {
    super(message);
    this.status = status;
  }
}

async function req<T>(method: string, url: string, body?: unknown): Promise<T> {
  const res = await fetch(url, {
    method,
    headers: body !== undefined ? { 'Content-Type': 'application/json' } : undefined,
    body: body !== undefined ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    let msg = res.statusText;
    try {
      const j = await res.json();
      if (j && j.error) msg = j.error;
    } catch {
      /* ignore */
    }
    throw new ApiError(res.status, msg);
  }
  if (res.status === 204) return undefined as T;
  return (await res.json()) as T;
}

export const get = <T>(url: string) => req<T>('GET', url);
export const post = <T>(url: string, body?: unknown) => req<T>('POST', url, body ?? {});
export const put = <T>(url: string, body?: unknown) => req<T>('PUT', url, body ?? {});
export const patch = <T>(url: string, body?: unknown) => req<T>('PATCH', url, body ?? {});
export const del = <T>(url: string) => req<T>('DELETE', url);

export const api = {
  bootstrap: () => get<Bootstrap>('/api/bootstrap'),
  fonts: () => get<string[]>('/api/fonts'),
  screens: () => get<Screen[]>('/api/screens'),
  quit: () => post('/api/app/quit'),
  openURL: (url: string) => post('/api/open-url', { url }),

  songbooks: () => get<Songbook[]>('/api/songbooks'),
  addSongbook: (name: string, info: string) => post<{ id: number }>('/api/songbooks', { name, info }),
  updateSongbook: (id: number, name: string, info: string) => put(`/api/songbooks/${id}`, { name, info }),
  deleteSongbook: (id: number) => del(`/api/songbooks/${id}`),
  songs: () => get<Song[]>('/api/songs'),
  song: (id: number) => get<Song>(`/api/songs/${id}`),
  stanzas: (text: string) => post<string[]>('/api/songs/stanzas', { text }),
  createSong: (song: Song, background?: string) => post<{ id: number }>('/api/songs', { song, background: background ?? '' }),
  updateSong: (song: Song, background?: string, clearBackground = false) =>
    put(`/api/songs/${song.id}`, { song, background: background ?? '', clearBackground }),
  deleteSong: (id: number) => del(`/api/songs/${id}`),
  searchSongs: (text: string, type: number) => post<{ ids: number[]; highlight: string }>('/api/songs/search', { text, type }),
  nextNumber: (songbook: number) => get<{ number: number }>(`/api/songs/next-number?songbook=${songbook}`),
  matchSong: (songbookName: string, number: number, title: string) =>
    post<{ id: number; found: boolean }>('/api/songs/match', { songbookName, number, title }),
  counts: () => get<CountRow[]>('/api/songs/counts'),
  resetCounts: (id?: number) => post('/api/songs/counts/reset', { id: id ?? 0 }),

  bibles: () => get<BibleInfo[]>('/api/bibles'),
  updateBible: (b: BibleInfo) => put(`/api/bibles/${b.id}`, b),
  deleteBible: (id: number) => del(`/api/bibles/${id}`),
  books: (id: number) => get<Book[]>(`/api/bibles/${id}/books`),
  chapter: (id: number, book: number, chapter: number) =>
    get<ChapterVerse[]>(`/api/bibles/${id}/chapter?book=${book}&chapter=${chapter}`),
  bibleRef: (id: number, ids: string) =>
    get<{ bookId: number; bookName: string; chapter: number; first: number; last: number }>(
      `/api/bibles/${id}/ref?ids=${encodeURIComponent(ids)}`,
    ),
  searchBible: (id: number, text: string, exact: boolean, fullWord: boolean, range: number, book: number, chapter: number) =>
    post<{ hits: SearchHit[]; highlight: string }>(`/api/bibles/${id}/search`, { text, exact, fullWord, range, book, chapter }),
  chapterText: (id: number, book: number, chapter: number) =>
    get<{ bible: string; book: string; chapter: number; verses: ChapterVerse[] }>(
      `/api/bibles/${id}/chapter-text?book=${book}&chapter=${chapter}`,
    ),

  announcements: () => get<Announcement[]>('/api/announcements'),
  createAnnouncement: (a: Announcement) => post<{ id: number }>('/api/announcements', a),
  updateAnnouncement: (a: Announcement) => put(`/api/announcements/${a.id}`, a),
  deleteAnnouncement: (id: number) => del(`/api/announcements/${id}`),
  announceSlides: (text: string) => post<string[]>('/api/announcements/slides', { text }),

  slideshows: () => get<SlideShow[]>('/api/slideshows'),
  slideshow: (id: number) => get<SlideShowFull>(`/api/slideshows/${id}`),
  saveSlideshow: (ss: SlideShowFull) => post<{ id: number }>('/api/slideshows', ss),
  deleteSlideshow: (id: number) => del(`/api/slideshows/${id}`),
  folderImages: (path: string, recursive = true, link = false) => post<Slide[]>('/api/slideshows/folder-images', { path, recursive, link }),
  addImages: (paths: string[], link = false) => post<Slide[]>('/api/slideshows/images', { paths, link }),

  media: () => get<MediaItem[]>('/api/media'),
  addMedia: (paths: string[]) => post<{ added: number }>('/api/media', { paths }),
  removeMedia: (path: string) => del(`/api/media?path=${encodeURIComponent(path)}`),

  themes: () => get<ThemeInfo[]>('/api/themes'),
  theme: (id: number) => get<Theme>(`/api/themes/${id}`),
  themePreview: (id: number) => get<RenderSlide>(`/api/themes/${id}/preview`),
  createTheme: (name: string) => post<{ id: number }>('/api/themes', { name }),
  copyTheme: (id: number, name: string) => post<{ id: number }>(`/api/themes/${id}/copy`, { name }),
  renameTheme: (id: number, name: string) => put(`/api/themes/${id}/info`, { name }),
  saveTheme: (t: Theme) => put(`/api/themes/${t.id}`, t),
  deleteTheme: (id: number) => del(`/api/themes/${id}`),
  themeImages: (id: number, kind: string, disp: number) => get<ThemeImage[]>(`/api/themes/${id}/images/${kind}/${disp}`),
  addThemeImages: (id: number, kind: string, disp: number, paths: string[]) => post<ThemeImage[]>(`/api/themes/${id}/images/${kind}/${disp}`, { paths }),
  selectThemeImage: (id: number, kind: string, disp: number, img: number) => post<ThemeImage[]>(`/api/themes/${id}/images/${kind}/${disp}/${img}/select`),
  renameThemeImage: (id: number, kind: string, disp: number, img: number, name: string) => put<ThemeImage[]>(`/api/themes/${id}/images/${kind}/${disp}/${img}`, { name }),
  removeThemeImage: (id: number, kind: string, disp: number, img: number) => del<ThemeImage[]>(`/api/themes/${id}/images/${kind}/${disp}/${img}`),

  settings: () => get<Settings>('/api/settings'),
  updateStatus: (refresh = false) => get<UpdateStatus>(`/api/update${refresh ? '?refresh=1' : ''}`),
  installUpdate: (tag: string, restart: boolean) => post('/api/update/install', { tag, restart }),
  restartApp: () => post('/api/update/restart', {}),
  remoteInfo: () => get<{ listen: string; urls: string[]; ips: string[] }>('/api/remote/info'),
  saveSettings: (s: Settings) => put('/api/settings', s),
  patchApp: (p: Partial<{ splitters: Record<string, number>; language: string; uiTheme: string; livePreview: boolean; slideshowOrder: number[]; bibleButtonOrder: string[]; moduleSource: string; moduleUrls: Record<string, string> }>) =>
    patch('/api/settings/app', p),

  // live
  showBible: (book: number, chapter: number, selection: number[]) => post('/api/live/show-bible', { book, chapter, selection }),
  showBibleRef: (verseIds: string) => post('/api/live/show-bible-ref', { verseIds }),
  showSong: (idOrSong: number | Song, row: number) =>
    post('/api/live/show-song', typeof idOrSong === 'number' ? { id: idOrSong, row } : { song: idOrSong, row }),
  showAnnouncement: (idOrA: number | Announcement, row: number) =>
    post('/api/live/show-announcement', typeof idOrA === 'number' ? { id: idOrA, row } : { announcement: idOrA, row }),
  showSlideshow: (ss: number | SlideShowFull, row: number) =>
    post('/api/live/show-slideshow', typeof ss === 'number' ? { id: ss, row } : { slideshow: ss, row }),
  showMedia: (m: MediaItem) => post('/api/live/show-media', m),
  showSchedule: (index: number) => post('/api/live/show-schedule', { index }),
  liveIndex: (index: number, force = false) => post('/api/live/index', { index, force }),
  previewSelection: (selection: number[]) => post<RenderSlide>('/api/live/preview-selection', { selection }),
  testDisplays: (slots: number[]) => post('/api/live/test-displays', { slots }),
  liveSelection: (selection: number[], project: boolean) => post('/api/live/selection', { selection, project }),
  multiVerse: (on: boolean) => post('/api/live/multiverse', { on }),
  next: () => post('/api/live/next'),
  prev: () => post('/api/live/prev'),
  show: () => post('/api/live/show'),
  hide: () => post('/api/live/hide'),
  clear: () => post('/api/live/clear'),
  previewSlide: (slide: unknown) => post('/api/live/preview-slide', slide),
  clearPreview: () => del('/api/live/preview-slide'),
  pause: () => post('/api/live/pause'),
  escape: () => post('/api/live/escape'),
  stop: () => post('/api/live/stop'),
  display: (on: boolean) => post('/api/live/display', { on }),
  video: (cmd: string, value = 0) => post('/api/live/video', { cmd, value }),

  // schedule
  schedule: () => get<ScheduleState>('/api/schedule'),
  scheduleAdd: (item: ScheduleItem) => post('/api/schedule/add', item),
  scheduleRemove: (index: number) => post('/api/schedule/remove', { index }),
  scheduleMove: (from: number, to: number) => post('/api/schedule/move', { From: from, To: to }),
  scheduleClear: () => post('/api/schedule/clear'),
  scheduleNew: () => post('/api/schedule/new'),
  scheduleClose: () => post('/api/schedule/close'),
  scheduleOpen: (path?: string) => post<{ ok?: boolean; cancelled?: boolean }>('/api/schedule/open', { path: path ?? '' }),
  scheduleSave: (as: boolean, path?: string) =>
    post<{ ok?: boolean; cancelled?: boolean; path?: string; download?: string }>('/api/schedule/save', { as, path: path ?? '' }),

  // manage
  dialogOpen: (title: string, filters: { name: string; pattern: string }[], multiple: boolean) =>
    post<{ paths: string[]; cancelled?: boolean }>('/api/dialog/open', { title, filters, multiple }),
  dialogFolder: (title: string) => post<{ path: string; name?: string; cancelled?: boolean }>('/api/dialog/folder', { title }),
  dialogSave: (title: string, filename: string, filters: { name: string; pattern: string }[]) =>
    post<{ path: string; cancelled?: boolean }>('/api/dialog/save', { title, filename, filters }),
  /** a theme as a zip file (theme.json and the pictures): the export opens a save dialog on the server */
  exportThemePack: (id: number) =>
    trackPicker(post<{ ok?: boolean; cancelled?: boolean; path?: string; name?: string; download?: string }>('/api/theme-pack/export', { id })),
  importThemePack: (path: string) => post<{ id: number; name: string }>('/api/theme-pack/import', { path }),
  importFiles: (kind: 'songbook' | 'bible' | 'theme', paths: string[]) => post<{ job: string }>(`/api/import/${kind}`, { paths }),
  exportFile: (kind: 'songbook' | 'bible' | 'theme', body: { id?: number; ids?: number[]; name?: string; path?: string }) =>
    // the export opens a native save dialog on the server
    trackPicker(post<{ ok?: boolean; cancelled?: boolean; path?: string; title?: string; download?: string }>(`/api/export/${kind}`, body)),
  moduleList: (kind: 'bible' | 'songbook' | 'theme', source = 'softprojector', url = '') =>
    get<Module[]>(`/api/modules/list?kind=${kind}&source=${source}&url=${encodeURIComponent(url)}`),
  moduleDownload: (kind: string, modules: Module[]) => post<{ job: string }>('/api/modules/download', { kind, modules }),
};

/** Uploads files from the browser (used when native dialogs are unavailable). */
export async function uploadFiles(files: FileList | File[]): Promise<string[]> {
  const fd = new FormData();
  Array.from(files).forEach((f, i) => fd.append('file' + i, f, f.name));
  const res = await fetch('/api/upload', { method: 'POST', body: fd });
  if (!res.ok) throw new ApiError(res.status, 'upload failed');
  const j = (await res.json()) as { paths: string[] };
  return j.paths;
}
