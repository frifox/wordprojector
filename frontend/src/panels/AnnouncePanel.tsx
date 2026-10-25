import { useEffect, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { focusLive } from '../lib/focus';
import { t } from '../i18n';
import { useRegisterPanel, useUI, GoLiveIcon } from '../lib/ui';
import Splitter from '../components/Splitter';
import { useData } from '../lib/hooks';
import type { Announcement, ScheduleItem } from '../lib/types';
import { confirmDialog, alertDialog } from '../components/Modal';

export default function AnnouncePanel({ active }: { active: boolean }) {
  const { settings } = useStore();
  const ui = useUI();
  const [list, reload] = useData<Announcement[]>('announcements', () => api.announcements());
  const [selectedId, setSelectedId] = useState(0);
  const [preview, setPreview] = useState<Announcement | null>(null);
  const [slides, setSlides] = useState<string[]>([]);
  const [slideIdx, setSlideIdx] = useState(0);
  const fromSchedule = useRef(false);

  const show = async (a: Announcement | null) => {
    setPreview(a);
    setSlides(a ? await api.announceSlides(a.text) : []);
    setSlideIdx(0);
  };
  const select = (a: Announcement) => {
    fromSchedule.current = false;
    setSelectedId(a.id);
    show(a);
  };
  const knownIds = useRef<Set<number> | null>(null);
  useEffect(() => {
    if (!list) return;
    // select an announcement that was just created
    if (knownIds.current) {
      const added = list.find((a) => !knownIds.current!.has(a.id));
      if (added) {
        knownIds.current = new Set(list.map((a) => a.id));
        select(added);
        return;
      }
    }
    knownIds.current = new Set(list.map((a) => a.id));
    if (preview && !fromSchedule.current) {
      const a = list.find((x) => x.id === preview.id);
      if (a) show(a);
      else {
        setPreview(null);
        setSlides([]);
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [list]);

  const goLive = () => {
    if (!preview) return;
    api.showAnnouncement(preview.id > 0 && !fromSchedule.current ? preview.id : preview, slideIdx);
    focusLive();
  };
  const addToSchedule = (a?: Announcement) => {
    const an = a || preview;
    if (!an) return;
    const item: ScheduleItem = { type: 'announce', name: an.title, announce: an };
    api.scheduleAdd(item);
  };
  const remove = async () => {
    if (!preview || preview.id <= 0) return alertDialog(t('No Announcement Selected'), t('No announcement has been selected to delete'), t('Please select an announcement to delete'));
    const r = await confirmDialog({
      title: t('Delete Announcement?'),
      message: t('Delete announcement: "{title}"?', { title: preview.title }),
      info: t('This action will permanently delete this announcement'),
      buttons: [{ label: t('Yes'), value: 'yes', primary: true }, { label: t('No'), value: 'no' }],
    });
    if (r === 'yes') {
      await api.deleteAnnouncement(preview.id);
      setPreview(null);
      setSelectedId(0);
      reload();
    }
  };
  useRegisterPanel('announce', {
    newItem: () => ui.openDialog({ kind: 'editAnnouncement', mode: 'new' }),
    edit: () => (preview ? ui.openDialog({ kind: 'editAnnouncement', mode: 'edit', announcement: preview }) : alertDialog(t('No Announcement Selected'), t('No announcement has been selected to edit'), t('Please select an announcement to edit'))),
    copy: () => (preview ? ui.openDialog({ kind: 'editAnnouncement', mode: 'copy', announcement: preview }) : alertDialog(t('No Announcement Selected'), t('No announcement has been selected to copy'), t('Please select an announcement to copy'))),
    remove,
    goLive,
    addToSchedule: () => addToSchedule(),
    print: () => {
      if (!preview) return alertDialog(t('No announcement selected'), t('No announcement has been selected to be printed.'), t('Please select a announcement to be printed.'));
      ui.openDialog({ kind: 'print', title: t('Print'), text: `${t('Announcements: {title}', { title: preview.title })}\n\n${preview.text}`, fontSize: 14 });
    },
    hasSelection: () => !!preview,
    showFromSchedule: (item) => {
      const it = item as ScheduleItem;
      if (it.announce) {
        fromSchedule.current = true;
        setSelectedId(0);
        show(it.announce);
      }
    },
  });

  return (
    <Splitter id="announce-main" initial={0.45} saved={settings.app.splitters} className="fill">
      <div className="col fill" style={{ gap: 4 }}>
        <span>{t('Announcements:')}</span>
        <div className={`table grow ${active ? 'active' : ''}`} tabIndex={0} onKeyDown={(e) => {
          if (!list) return;
          const idx = list.findIndex((a) => a.id === selectedId);
          if (e.key === 'ArrowDown' && idx < list.length - 1) { select(list[idx + 1]); e.preventDefault(); }
          else if (e.key === 'ArrowUp' && idx > 0) { select(list[idx - 1]); e.preventDefault(); }
          else if (e.key === 'Enter' && idx >= 0) addToSchedule(list[idx]);
        }}>
          <table>
            <thead><tr><th>{t('Title')}</th></tr></thead>
            <tbody>
              {(list || []).map((a) => (
                <tr key={a.id} className={a.id === selectedId ? 'sel' : ''} onClick={() => select(a)} onDoubleClick={() => { addToSchedule(a); select(a); }}>
                  <td>{a.title}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </div>
      <div className="col fill" style={{ gap: 4 }}>
        <div className="row">
          <button className="primary golive" onClick={goLive} disabled={!preview} title={t('Quickly display announcement')} aria-label={t('Go Live')}><GoLiveIcon /></button>
          <span style={{ fontWeight: 600 }}>{preview ? preview.title : t('Announcement Preview:')}</span>
        </div>
        <div className="list grow" tabIndex={0} onKeyDown={(e) => {
          if (e.key === 'ArrowDown') { setSlideIdx((i) => Math.min(slides.length - 1, i + 1)); e.preventDefault(); }
          else if (e.key === 'ArrowUp') { setSlideIdx((i) => Math.max(0, i - 1)); e.preventDefault(); }
          else if (e.key === 'Enter') goLive();
        }}>
          {slides.map((s, i) => (
            <div key={i} className={`item spaced ${i === slideIdx ? 'sel' : ''}`} onClick={() => setSlideIdx(i)} onDoubleClick={() => { setSlideIdx(i); api.showAnnouncement(preview && preview.id > 0 && !fromSchedule.current ? preview.id : preview!, i); focusLive(); }}>{s}</div>
          ))}
        </div>
      </div>
    </Splitter>
  );
}
