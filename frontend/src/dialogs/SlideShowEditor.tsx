import { useRef, useState } from 'react';
import Modal, { alertDialog, useDiscardGuard } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { Slide, SlideShowFull } from '../lib/types';
import { pickFiles, imageFilter } from '../lib/files';
import { slideURL } from '../panels/PicturesPanel';

export default function SlideShowEditor({ slideshow, onClose }: { slideshow?: SlideShowFull; onClose: () => void }) {
  const { boot } = useStore();
  const [name, setName] = useState(slideshow?.name || '');
  const [info, setInfo] = useState(slideshow?.info || '');
  const [slides, setSlides] = useState<Slide[]>(slideshow?.slides || []);
  const [idx, setIdx] = useState(0);
  const baseline = useRef(JSON.stringify([name, info, slides]));
  const requestClose = useDiscardGuard(JSON.stringify([name, info, slides]) !== baseline.current, onClose, () => save());
  const cur = slides[idx];
  const add = async () => {
    const paths = await pickFiles(boot.desktop, t('Select Images to Open'), [imageFilter], true);
    if (!paths.length) return;
    const added = await api.addImages(paths);
    setSlides((s) => [...s, ...added]);
  };
  const move = (d: number) => {
    const j = idx + d;
    if (j < 0 || j >= slides.length) return;
    setSlides((s) => {
      const n = [...s];
      [n[idx], n[j]] = [n[j], n[idx]];
      return n;
    });
    setIdx(j);
  };
  const save = async () => {
    if (!slides.length) {
      onClose();
      return;
    }
    if (!name.trim()) {
      await alertDialog(t('Slide show title is missing'), t('Slide show title cannot be left empty.\nPlease enter a title.'));
      return;
    }
    await api.saveSlideshow({ id: slideshow?.id || 0, name: name.trim(), info, slides });
    onClose();
  };
  return (
    <Modal title={slideshow ? t('Edit Slide Show') : t('New Slide Show')} onClose={requestClose} width={760} height="85vh"
      buttons={<><button onClick={requestClose}>{t('Cancel')}</button><button className="primary" onClick={save}>{t('Save')}</button></>}>
      <div className="form-grid">
        <span>{t('Title:')}</span>
        <input type="text" autoFocus value={name} onChange={(e) => setName(e.target.value)} />
        <span style={{ alignSelf: 'flex-start' }}>{t('Information:')}</span>
        <textarea rows={2} value={info} onChange={(e) => setInfo(e.target.value)} />
      </div>
      <div className="row" style={{ alignItems: 'stretch' }}>
        <div style={{ width: 400, height: 300, background: '#222', display: 'flex', alignItems: 'center', justifyContent: 'center', border: '1px solid var(--border)', flex: 'none' }}>
          {cur ? <img src={slideURL(cur, 'prev')} alt="" style={{ maxWidth: 400, maxHeight: 300 }} /> : <span style={{ color: '#aaa' }}>{t('Picture Preview')}</span>}
        </div>
        <div className="col" style={{ gap: 4 }}>
          <button onClick={add}><img className="icon" src="/icons/add.png" alt="" />{t('Add Images...')}</button>
          <button disabled={!cur} onClick={() => { setSlides((s) => s.filter((_, i) => i !== idx)); setIdx((i) => Math.max(0, Math.min(i, slides.length - 2))); }}><img className="icon" src="/icons/remove.png" alt="" />{t('Remove Image')}</button>
          <button disabled={idx <= 0} onClick={() => move(-1)}><img className="icon" src="/icons/up.png" alt="" />{t('Move Up')}</button>
          <button disabled={idx >= slides.length - 1} onClick={() => move(1)}><img className="icon" src="/icons/down.png" alt="" />{t('Move Down')}</button>
          <span className="muted">{t('Preview slide: ')}{cur?.name || ''}</span>
        </div>
      </div>
      <div className="list grow thumb-list" style={{ minHeight: 130 }}>
        {slides.map((s, i) => (
          <div key={s.id || s.temp} className={`thumb ${i === idx ? 'sel' : ''}`} onClick={() => setIdx(i)}>
            <img src={slideURL(s, 'small')} alt={s.name} />
          </div>
        ))}
      </div>
    </Modal>
  );
}
