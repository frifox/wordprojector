import { useEffect, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { focusLive } from '../lib/focus';
import { t } from '../i18n';
import { useRegisterPanel, GoLiveIcon } from '../lib/ui';
import Splitter from '../components/Splitter';
import { useData } from '../lib/hooks';
import type { MediaItem, ScheduleItem } from '../lib/types';
import { alertDialog } from '../components/Modal';
import { pickFiles, mediaFilter } from '../lib/files';

export default function MediaPanel({ active }: { active: boolean }) {
  const { settings, boot } = useStore();
  const [list, reload] = useData<MediaItem[]>('media', () => api.media());
  const [current, setCurrent] = useState<MediaItem | null>(null);
  const [hasVideo, setHasVideo] = useState(false);
  const [error, setError] = useState('');
  const videoRef = useRef<HTMLVideoElement>(null);
  const fromSchedule = useRef(false);

  const select = (m: MediaItem) => {
    fromSchedule.current = false;
    setCurrent(m);
    setError('');
  };
  useEffect(() => {
    if (current && list && !fromSchedule.current && !list.find((m) => m.path === current.path)) setCurrent(null);
  }, [list, current]);

  const goLive = () => {
    if (!current) return;
    videoRef.current?.pause();
    api.showMedia(current);
    focusLive();
  };
  const addToSchedule = () => {
    if (!current) return;
    const item: ScheduleItem = { type: 'media', name: current.name, media: current };
    api.scheduleAdd(item);
  };
  const add = async () => {
    const paths = await pickFiles(boot.desktop, t('Select Music/Video Files to Open'), [mediaFilter], true);
    if (paths.length) {
      await api.addMedia(paths);
      reload();
    }
  };
  const remove = async () => {
    if (!current) return alertDialog(t('No media selected'), t('No media item has been selected to delete.'), t('Please select a media item to delete.'));
    await api.removeMedia(current.path);
    setCurrent(null);
    reload();
  };
  useRegisterPanel('media', {
    newItem: add,
    remove,
    goLive,
    addToSchedule,
    hasSelection: () => !!current,
    showFromSchedule: (item) => {
      const it = item as ScheduleItem;
      if (it.media) {
        fromSchedule.current = true;
        setCurrent(it.media);
      }
    },
  });

  const src = current ? `/api/media/file?path=${encodeURIComponent(current.path)}` : '';
  return (
    <Splitter id="media-main" initial={0.4} saved={settings.app.splitters} className="fill">
      <div className="col fill" style={{ gap: 4 }}>
        <span>{t('Media Library:')}</span>
        <div className={`list grow ${active ? 'active' : ''}`} tabIndex={0} onKeyDown={(e) => {
          if (!list) return;
          const idx = list.findIndex((m) => m.path === current?.path);
          if (e.key === 'ArrowDown' && idx < list.length - 1) { select(list[idx + 1]); e.preventDefault(); }
          else if (e.key === 'ArrowUp' && idx > 0) { select(list[idx - 1]); e.preventDefault(); }
        }}>
          {(list || []).map((m) => (
            <div key={m.path} className={`item ${m.path === current?.path ? 'sel' : ''}`} onClick={() => select(m)} onDoubleClick={() => { select(m); setTimeout(goLive, 100); }} title={m.path}>{m.name}</div>
          ))}
        </div>
        <div className="row">
          <button onClick={add}><img className="icon" src="/icons/video_add.png" alt="" />{t('Add Media Files...')}</button>
          <button onClick={remove} disabled={!current}><img className="icon" src="/icons/video_remove.png" alt="" />{t('Remove')}</button>
        </div>
      </div>
      <div className="col fill" style={{ gap: 4 }}>
        <div className="row">
          <button className="primary golive" onClick={goLive} disabled={!current} title={t('Go Live (F5)')} aria-label={t('Go Live (F5)')}><GoLiveIcon /></button>
          <span style={{ fontWeight: 600 }}>{current?.name}</span>
        </div>
        <div className="grow" style={{ background: '#000', border: '1px solid var(--border)', position: 'relative', minHeight: 120, display: 'flex', alignItems: 'center', justifyContent: 'center' }}>
          {current ? (
            <video
              ref={videoRef}
              key={src}
              src={src}
              controls
              style={{ width: '100%', height: '100%', objectFit: 'contain' }}
              onLoadedMetadata={(e) => setHasVideo(e.currentTarget.videoWidth > 0)}
              onError={() => setError(t('ERROR Playing media file') + '\n' + t(' - Unsupported media format') + '\n' + t(' - Media file no longer exists or invalid path to file'))}
            />
          ) : (
            <span style={{ color: '#ccc' }}>{t('No media')}</span>
          )}
          {error && <div style={{ position: 'absolute', inset: 0, color: '#ff5555', padding: 10, whiteSpace: 'pre-wrap', background: 'rgba(0,0,0,0.7)' }}>{error}</div>}
          {!hasVideo && current && !error && <div style={{ position: 'absolute', top: 8, left: 8, color: '#49fff9' }}>{current.name}</div>}
        </div>
      </div>
    </Splitter>
  );
}
