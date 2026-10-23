import React, { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState } from 'react';
import { api } from './api';
import Socket, { type WsMessage } from './ws';
import type { Bootstrap, LiveState, ProgressMsg, ScheduleState, Settings, Theme, ThemeInfo, BibleInfo, Songbook } from './types';
import { setLanguage } from '../i18n';

export interface Store {
  boot: Bootstrap;
  live: LiveState;
  schedule: ScheduleState;
  settings: Settings;
  theme: Theme;
  themes: ThemeInfo[];
  bibles: BibleInfo[];
  songbooks: Songbook[];
  operatorBible: number;
  connected: boolean;
  /** data invalidation counters per data set */
  dataVersion: Record<string, number>;
  progress: Record<string, ProgressMsg & { log: string[] }>;
  clearProgress: (job: string) => void;
  reloadSettings: () => Promise<void>;
  socket: Socket;
  lang: number;
}

const StoreCtx = createContext<Store | null>(null);

export function useStore(): Store {
  const s = useContext(StoreCtx);
  if (!s) throw new Error('no store');
  return s;
}

export function StoreProvider({ boot, children }: { boot: Bootstrap; children: React.ReactNode }) {
  const [live, setLive] = useState<LiveState>(boot.state);
  const [schedule, setSchedule] = useState<ScheduleState>({ items: [], file: '', saved: true });
  const [settings, setSettings] = useState<Settings>(boot.settings);
  const [theme, setTheme] = useState<Theme>(boot.theme);
  const [themes, setThemes] = useState<ThemeInfo[]>(boot.themes);
  const [bibles, setBibles] = useState<BibleInfo[]>(boot.bibles);
  const [songbooks, setSongbooks] = useState<Songbook[]>(boot.songbooks);
  const [operatorBible, setOperatorBible] = useState(boot.operatorBible);
  const [connected, setConnected] = useState(false);
  const [dataVersion, setDataVersion] = useState<Record<string, number>>({});
  const [progress, setProgress] = useState<Record<string, ProgressMsg & { log: string[] }>>({});
  const [lang, setLang] = useState(0);
  const socketRef = useRef<Socket | null>(null);
  if (!socketRef.current) socketRef.current = new Socket('operator');
  const socket = socketRef.current;

  const reloadSettings = useCallback(async () => {
    const b = await api.bootstrap();
    setSettings(b.settings);
    setTheme(b.theme);
    setThemes(b.themes);
    setBibles(b.bibles);
    setSongbooks(b.songbooks);
    setOperatorBible(b.operatorBible);
  }, []);

  useEffect(() => {
    setLanguage(boot.settings.app.language || boot.settings.main.uiTranslation || 'en');
    setLang((n) => n + 1);
  }, [boot.settings.app.language, boot.settings.main.uiTranslation]);

  useEffect(() => {
    socket.onStatus = (c) => setConnected(c);
    const off = socket.on((m: WsMessage) => {
      switch (m.type) {
        case 'state':
          setLive(m.state);
          break;
        case 'schedule':
          setSchedule({ items: m.items, file: m.file, saved: m.saved });
          break;
        case 'data':
          setDataVersion((v) => ({ ...v, [m.what]: (v[m.what] || 0) + 1 }));
          if (m.what === 'settings' || m.what === 'themes' || m.what === 'bibles' || m.what === 'songbooks') {
            reloadSettings();
          }
          break;
        case 'videoStatus':
          setLive((l) => ({ ...l, video: m.video }));
          break;
        case 'progress':
          setProgress((p) => {
            const prev = p[m.job] || { type: 'progress', job: m.job, log: [] };
            const log = m.text ? [...prev.log, m.text] : prev.log;
            if (m.error && m.done) log.push('Error: ' + m.error);
            return { ...p, [m.job]: { ...prev, ...m, log } };
          });
          if (m.done) {
            setDataVersion((v) => ({ ...v, all: (v.all || 0) + 1 }));
            reloadSettings();
          }
          break;
      }
    });
    socket.start();
    return () => {
      off();
      socket.stop();
    };
  }, [socket, reloadSettings]);

  const clearProgress = useCallback((job: string) => {
    setProgress((p) => {
      const n = { ...p };
      delete n[job];
      return n;
    });
  }, []);

  const value = useMemo<Store>(
    () => ({
      boot, live, schedule, settings, theme, themes, bibles, songbooks, operatorBible, connected, dataVersion,
      progress, clearProgress, reloadSettings, socket, lang,
    }),
    [boot, live, schedule, settings, theme, themes, bibles, songbooks, operatorBible, connected, dataVersion, progress, clearProgress, reloadSettings, socket, lang],
  );
  return <StoreCtx.Provider value={value}>{children}</StoreCtx.Provider>;
}
