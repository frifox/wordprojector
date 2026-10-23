import { useRef, useState } from 'react';
import Modal, { alertDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { Announcement } from '../lib/types';
import { HighlightTextarea } from '../components/Pickers';

function blank(): Announcement {
  return {
    id: 0, title: '', usePrivate: false, useAuto: true, loop: true, slideTime: 10, useBackground: false, backgroundPath: '',
    font: { family: 'Arial', pointSize: 28, bold: false, italic: false }, color: '#ffffff', alignV: 0, alignH: 1,
    text: t('Announce\n - Text of the announcement goes here\n\nSlide\n - Text of the announcement goes here\nYou can have both Announce or Slide as announcement block titles.'),
  };
}

export default function EditAnnouncementDialog({ mode, announcement, onClose }: { mode: 'new' | 'edit' | 'copy'; announcement?: Announcement; onClose: () => void }) {
  const { boot } = useStore();
  const [a, setA] = useState<Announcement>(() => (announcement && mode !== 'new' ? { ...announcement, id: mode === 'copy' ? 0 : announcement.id } : blank()));
  const baseline = useRef(JSON.stringify(a));
  const requestClose = useDiscardGuard(JSON.stringify(a) !== baseline.current, onClose, () => save());
  const set = (p: Partial<Announcement>) => setA((x) => ({ ...x, ...p }));
  const save = async () => {
    const title = a.title.trim().replace(/\s+/g, ' ');
    if (!title) {
      await alertDialog(t('Announcement title is missing'), t('Announcement title cannot be left empty.\nPlease enter announcement title.'));
      return;
    }
    const an = { ...a, title };
    try {
      if (an.id > 0) await api.updateAnnouncement(an);
      else await api.createAnnouncement(an);
    } catch (e) {
      await alertDialog(t('Error'), String((e as Error).message || e));
      return;
    }
    onClose();
  };
  return (
    <Modal title={mode === 'edit' ? t('Edit Announcement') : t('New Announcement')} onClose={requestClose} width={640} height="80vh"
      buttons={<><button onClick={requestClose}>{t('Cancel')}</button><button className="primary" onClick={save}>{t('Save')}</button></>}>
      <div className="col fill" style={{ gap: 6 }} onKeyDown={(e) => { if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') { e.preventDefault(); save(); } }}>
        <div className="row">
          <span>{t('Title:')}</span>
          <input type="text" className="grow" autoFocus value={a.title} onChange={(e) => set({ title: e.target.value })} />
          <span className="muted">ID {a.id}</span>
        </div>
        <div className="row">
          <label className="chk"><input type="checkbox" checked={a.useAuto} onChange={(e) => set({ useAuto: e.target.checked })} />{t('Timed slides, every')}</label>
          <input type="number" min={1} max={3600} value={a.slideTime} disabled={!a.useAuto} style={{ width: 70 }} onChange={(e) => set({ slideTime: Number(e.target.value) || 10 })} />
          <span>{t('seconds')}</span>
          <label className="chk"><input type="checkbox" checked={a.loop} disabled={!a.useAuto} onChange={(e) => set({ loop: e.target.checked })} />{t('Loop')}</label>
        </div>
        <HighlightTextarea value={a.text} onChange={(v) => set({ text: v })} titleWords={boot.titles.announce} style={{ flex: 1, minHeight: 240 }} />
      </div>
    </Modal>
  );
}
