// Types mirroring the Go JSON API.

export interface Font {
  family: string;
  pointSize: number;
  bold: boolean;
  italic: boolean;
  underline?: boolean;
  strikeout?: boolean;
}

export interface Songbook {
  id: number;
  name: string;
  info: string;
}

export interface Song {
  id: number;
  songbookId: number;
  songbookName: string;
  number: number;
  title: string;
  category: number;
  tune: string;
  wordsBy: string;
  musicBy: string;
  text?: string;
  notes: string;
  usePrivate: boolean;
  alignV: number;
  alignH: number;
  color: string;
  font: Font;
  infoColor: string;
  infoFont: Font;
  endingColor: string;
  endingFont: Font;
  useBackground: boolean;
  backgroundName: string;
  hasBackground: boolean;
  count: number;
  date: string;
}

export interface CountRow {
  id: number;
  songbook: string;
  number: number;
  title: string;
  count: number;
  date: string;
}

export interface BibleInfo {
  id: number;
  name: string;
  abbr: string;
  info: string;
  rtl: boolean;
}

export interface Book {
  id: number;
  name: string;
  chapters: number;
}

export interface ChapterVerse {
  num: number;
  text: string;
  ids: string;
}

export interface SearchHit {
  book: string;
  bookId: number;
  chapter: number;
  verse: number;
  text: string;
  display: string;
}

export interface Announcement {
  id: number;
  title: string;
  text: string;
  usePrivate: boolean;
  useAuto: boolean;
  loop: boolean;
  slideTime: number;
  useBackground: boolean;
  backgroundPath: string;
  font: Font;
  color: string;
  alignV: number;
  alignH: number;
}

export interface SlideShow {
  id: number;
  name: string;
  info: string;
  /** the day it was made, YYYY-MM-DD */
  created?: string;
}

export interface Slide {
  id: number;
  order: number;
  name: string;
  path: string;
  /** the picture is shown from where it lies on disk (path); only thumbnails are kept */
  linked?: boolean;
  temp?: string;
  width: number;
  height: number;
}

export interface SlideShowFull extends SlideShow {
  slides: Slide[];
}

export interface MediaItem {
  path: string;
  name: string;
}

export interface ThemeInfo {
  id: number;
  name: string;
  builtin?: boolean;
}

export interface TextSettings {
  useShadow: boolean;
  useBlurShadow: boolean;
  textFont: Font;
  textColor: string;
  alignV: number;
  alignH: number;
  /** width of the characters, percent of normal */
  scaleH: number;
}

export interface PassiveSettings {
  /** how the image fills the screen: 0 scale (stretch), 1 fill (cover), 2 fit (contain) */
  backgroundFit: number;
  useBackground: boolean;
  /** the picture of the theme's own pool shown when nothing is projected, 0 for none */
  imageId: number;
}

export interface BibleSettings extends TextSettings {
  captionFont: Font;
  captionColor: string;
  captionAlign: number;
  captionPosition: number;
  /** width of the caption characters, percent of normal */
  captionScaleH: number;
  screenUse: number;
  screenPosition: number;
  addBkColorToText: boolean;
  textRecBkColor: string;
  /** space above and below the text inside its background color */
  bkPadding: number;
  /** how opaque that color is, percent */
  bkOpacity: number;
}

export interface SongSettings extends TextSettings {
  showStanzaTitle: boolean;
  showKey: boolean;
  showNumber: boolean;
  infoColor: string;
  infoFont: Font;
  infoAlign: number;
  infoScaleH: number;
  showEnding: boolean;
  endingColor: string;
  endingFont: Font;
  endingType: number;
  endingPosition: number;
  endingScaleH: number;
  screenUse: number;
  screenPosition: number;
  addBkColorToText: boolean;
  textRecBkColor: string;
  /** space above and below the text inside its background color */
  bkPadding: number;
  /** how opaque that color is, percent */
  bkOpacity: number;
}

/** The look of Bible, song and announcement slides, the same on every display. */
export interface Theme extends ThemeInfo {
  /** the color of the screen behind the text */
  backgroundColor: string;
  /** how opaque that color is, in percent; less lets the picture of the theme show through */
  backgroundOpacity: number;
  bible: BibleSettings;
  song: SongSettings;
  /** the picture shown when nothing is projected */
  passive: PassiveSettings;
}

export interface DisplayControls {
  buttonSize: number;
  alignV: number;
  alignH: number;
  opacity: number;
}

export interface GeneralSettings {
  displayIsOnTop: boolean;
  displayOnStartUp: boolean;
  currentThemeId: number;
  displayScreen: number;
  displayScreen2: number;
  displayScreen3: number;
  displayScreen4: number;
  displayScreensExtra?: number[];
  displayControls: DisplayControls;
}

export interface BibleVersions {
  primary: string;
  secondary: string;
  trinary: string;
  operator: string;
}

export interface SlideSettings {
  expandSmall: boolean;
  fitType: number;
  resize: boolean;
  boundType: number;
  boundWidth: number;
}

export interface AppSettings {
  window: { X: number; Y: number; Width: number; Height: number };
  splitters: Record<string, number>;
  language: string;
  remote?: { public: boolean; port: number };
  transition?: { cut: boolean; fadeMillis: number };
  hideDisplayControls?: boolean;
  livePreview: boolean;
  uiTheme: string;
  /** the font of the operator window; empty is the system font */
  uiFont?: string;
  defaultSongbook?: number;
  defaultBible?: number;
  slideshowOrder?: number[];
  /** the look for a newer release at start is on unless this is set */
  /** the order of the Bible version buttons (Bible ids) */
  bibleButtonOrder?: string[];
  /** what Books > Import from Cloud last used */
  moduleSource?: string;
  moduleUrls?: Record<string, string>;
  skipUpdateCheck?: boolean;
  /** a newer release is installed by itself (used from the next start) */
  autoInstallUpdates?: boolean;
  /** the keys the user chose for the program's actions (see lib/hotkeys.ts) */
  hotkeys?: Record<string, string>;
}

export interface UpdateRelease {
  tag: string;
  name: string;
  published: string;
  notes: string;
  prerelease: boolean;
  url: string;
  /** has a build for this computer */
  installable: boolean;
  /** the commit the tag points to (only looked up for the release named like the running version) */
  commit?: string;
}

export interface UpdateStatus {
  current: string;
  /** the commit the running program was built from, when known */
  commit?: string;
  supported: boolean;
  reason?: string;
  repo: string;
  releases: UpdateRelease[];
  latest: string;
  /** the latest release is newer than the running program (and not installed yet) */
  newer: boolean;
  checkedAt: string;
  error?: string;
  installing?: string;
  progress?: string;
  /** installed and waiting for a restart */
  pending?: string;
  canRestart: boolean;
}

export interface Settings {
  general: GeneralSettings;
  main: { uiTranslation: string; isWindowMaximized: boolean };
  bible: BibleVersions[];
  slides: SlideSettings;
  app: AppSettings;
}

export interface Screen {
  index: number;
  name: string;
  x: number;
  y: number;
  width: number;
  height: number;
  isPrimary: boolean;
}

// ---- live state ----

/** one background image in a theme's list */
export interface ThemeImage {
  id: number;
  name: string;
  selected: boolean;
}

export interface Background {
  type: 'color' | 'image';
  color?: string;
  url?: string;
  /** how an image fills the screen: cover, contain or fill (stretch, the default) */
  fit?: 'cover' | 'contain' | 'fill';
  /** how see-through a color is, in percent (0 or missing is solid) */
  clear?: number;
  /** what shows through a see-through color: the picture of the theme, else black */
  under?: Background;
}

export interface TextStyle {
  font: Font;
  color: string;
  shadow: boolean;
  blur: boolean;
  alignV: number;
  alignH: number;
  scaleH?: number;
}

export interface Passage {
  text: string;
  caption: string;
}

export interface BibleSlide {
  parts: Passage[];
  text: TextStyle;
  captionFont: Font;
  captionColor: string;
  captionAlign: number;
  captionPosition: number;
  captionScaleH?: number;
  screenUse: number;
  screenPosition: number;
  addBk: boolean;
  recBk: string;
  bkPadding?: number;
  bkOpacity?: number;
}

export interface SongSlide {
  text: string;
  stanzaTitle: string;
  number: number;
  tune: string;
  isLast: boolean;
  style: TextStyle;
  showTitle: boolean;
  showKey: boolean;
  showNumber: boolean;
  infoFont: Font;
  infoColor: string;
  infoAlign: number;
  infoScaleH?: number;
  showEnding: boolean;
  endingFont: Font;
  endingColor: string;
  endingType: number;
  endingPosition: number;
  endingScaleH?: number;
  screenUse: number;
  screenPosition: number;
  addBk: boolean;
  recBk: string;
  bkPadding?: number;
  bkOpacity?: number;
  /** text of every stanza, so they are all fitted together */
  stanzas?: string[];
  wordsBy: string;
  musicBy: string;
}

export interface AnnounceSlide {
  text: string;
  style: TextStyle;
}

export interface PictureSlide {
  url: string;
  width: number;
  height: number;
  expandSmall: boolean;
  fitType: number;
}

export interface VideoSlide {
  url: string;
  name: string;
  muted: boolean;
}

export interface RenderSlide {
  kind: 'passive' | 'clear' | 'bible' | 'song' | 'announce' | 'picture' | 'video' | 'test';
  test?: { disp: number; screen: number; millis: number; width: number; height: number };
  fade: boolean;
  fadeMillis?: number;
  background: Background;
  bible?: BibleSlide;
  song?: SongSlide;
  announce?: AnnounceSlide;
  picture?: PictureSlide;
  video?: VideoSlide;
}

export interface DisplayState {
  disp: number;
  screen: number;
  /** pixel size of the screen (0 when unknown) */
  width?: number;
  height?: number;
  slide: RenderSlide;
  /** what the display would show if the presentation were running */
  preview?: RenderSlide;
}

export interface LiveItem {
  text: string;
  image?: string;
}

export interface VideoState {
  position: number;
  duration: number;
  playing: boolean;
  ended: boolean;
  volume: number;
  muted: boolean;
}

export interface Controls {
  visible: boolean;
  buttonSize: number;
  alignV: number;
  alignH: number;
  opacity: number;
}

export interface LiveState {
  version: number;
  showing: boolean;
  cleared: boolean;
  displayOn: boolean;
  /** the test pattern is on the displays */
  test: boolean;
  singleScreen: boolean;
  type: '' | 'bible' | 'song' | 'announce' | 'picture' | 'video';
  title: string;
  notes: string;
  items: LiveItem[];
  index: number;
  selection: number[];
  multiVerse: boolean;
  displays: DisplayState[];
  video: VideoState;
  controls: Controls;
}

export interface BibleRef {
  verseIds: string;
  caption: string;
  captionLong: string;
}

export interface ScheduleItem {
  type: 'bible' | 'song' | 'slideshow' | 'media' | 'announce';
  name: string;
  scid?: number;
  bible?: BibleRef;
  song?: Song;
  slideshow?: SlideShowFull;
  media?: MediaItem;
  announce?: Announcement;
}

export interface ScheduleState {
  items: ScheduleItem[];
  file: string;
  saved: boolean;
}

export interface Bootstrap {
  version: string;
  desktop: boolean;
  dataDir: string;
  settings: Settings;
  theme: Theme;
  themes: ThemeInfo[];
  bibles: BibleInfo[];
  songbooks: Songbook[];
  screens: Screen[] | null;
  state: LiveState;
  categories: string[];
  operatorBible: number;
  titles: { verse: string[]; refrain: string[]; slide: string[]; announce: string[] };
}

export interface Module {
  name: string;
  link: string;
  size: number;
}

export interface ProgressMsg {
  type: 'progress';
  job: string;
  cur?: number;
  max?: number;
  msg?: string;
  text?: string;
  total?: number;
  totalMax?: number;
  done?: boolean;
  error?: string;
}
