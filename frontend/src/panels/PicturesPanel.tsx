import { useEffect, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { focusLive } from '../lib/focus';
import { t } from '../i18n';
import { useRegisterPanel, useUI } from '../lib/ui';
import Splitter from '../components/Splitter';
import { useData } from '../lib/hooks';
import type { SlideShow, SlideShowFull, Slide, ScheduleItem } from '../lib/types';
import { confirmDialog, alertDialog } from '../components/Modal';
import { pickFiles, pickFolder, imageFilter, type FolderPick } from '../lib/files';
import NameInfoDialog from '../dialogs/NameInfoDialog';
import { usePickerActive } from '../lib/picker';
import { startScheduleDrag } from '../lib/dnd';
import PhotoViewer from '../components/PhotoViewer';
import FolderShowDialog from '../dialogs/FolderShowDialog';

export function slideURL(s: Slide, size: 'full' | 'small' | 'prev') {
  return s.id > 0 ? `/api/slides/${s.id}/image?size=${size}` : `/api/blob/${encodeURIComponent(s.temp || '')}?size=${size}`;
}

const PlusIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M12 5l0 14" /><path d="M5 12l14 0" /></svg>;
const MinusIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M5 12l14 0" /></svg>;
const FolderPlusIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M3 7a2 2 0 0 1 2-2h4l2 2h8a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2z" /><path d="M12 10.5v5" /><path d="M9.5 13h5" /></svg>;
const PencilIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4" /><path d="M13.5 6.5l4 4" /></svg>;
const TrashIcon = () => <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>;

/** Slides tab: slide shows on the left (add, remove, drag to re-order), the photos of the selected one on the right. */
export default function PicturesPanel({ active }: { active: boolean }) {
  const { settings, boot, reloadSettings } = useStore();
  const ui = useUI();
  const [shows, reload] = useData<SlideShow[]>('slideshows', () => api.slideshows());
  const [selectedId, setSelectedId] = useState(0);
  const [current, setCurrent] = useState<SlideShowFull | null>(null);
  const [slides, setSlides] = useState<Slide[]>([]);
  const [picked, setPicked] = useState<number[]>([]); // selected photo positions
  const anchor = useRef(0);
  const [creating, setCreating] = useState(false);
  const picking = usePickerActive(); // a native file dialog is open
  const [viewing, setViewing] = useState(false); // full window photo view
  const [creatingFolder, setCreatingFolder] = useState(false);
  const fromSchedule = useRef(false); // a copy that came from the schedule: edits stay local to it

  // ---- slide shows ------------------------------------------------------------------------------------------
  const order = settings.app.slideshowOrder || [];
  const ordered = [...(shows || [])].sort((a, b) => {
    const ia = order.indexOf(a.id);
    const ib = order.indexOf(b.id);
    return (ia < 0 ? 1e9 + a.id : ia) - (ib < 0 ? 1e9 + b.id : ib);
  });

  const load = async (id: number) => {
    const ss = await api.slideshow(id);
    setCurrent(ss);
    setSlides(ss.slides);
    setPicked(ss.slides.length ? [0] : []);
    anchor.current = 0;
  };
  const select = (s: SlideShow) => {
    fromSchedule.current = false;
    setSelectedId(s.id);
    load(s.id);
  };
  const clear = () => {
    setSelectedId(0);
    setCurrent(null);
    setSlides([]);
    setPicked([]);
  };
  useEffect(() => {
    if (!shows || !current || fromSchedule.current) return;
    if (!shows.find((s) => s.id === current.id)) clear();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [shows]);

  const [renaming, setRenaming] = useState<SlideShow | null>(null);
  const renameShow = async (name: string) => {
    const target = renaming;
    setRenaming(null);
    if (!target || name === target.name) return;
    const ss = await api.slideshow(target.id);
    await api.saveSlideshow({ ...ss, name });
    await reload();
    if (current?.id === target.id) load(target.id);
  };
  const createShow = async (name: string, info: string) => {
    setCreating(false);
    const r = await api.saveSlideshow({ id: 0, name, info, slides: [] });
    await reload();
    fromSchedule.current = false;
    setSelectedId(r.id);
    load(r.id);
  };
  const folderSlides = (f: FolderPick) => (f.dir ? api.folderImages(f.dir) : api.addImages(f.paths ?? []));
  const createShowFromFolder = async (f: FolderPick, name: string) => {
    const imgs = await folderSlides(f);
    if (!imgs.length) return false;
    setCreatingFolder(false);
    const r = await api.saveSlideshow({ id: 0, name, info: '', slides: imgs });
    await reload();
    fromSchedule.current = false;
    setSelectedId(r.id);
    load(r.id);
    return true;
  };
  // asks first, then deletes that slide show (the one in the list under the pointer, or the selected one)
  const deleteShow = async (show: { id: number; name: string }) => {
    const r = await confirmDialog({
      title: t('Delete slideshow?'),
      message: t('Delete slideshow: "{title}"?', { title: show.name }),
      info: t('This action will permanently delete this slideshow'),
      buttons: [{ label: t('Yes'), value: 'yes', primary: true }, { label: t('No'), value: 'no' }],
    });
    if (r !== 'yes') return;
    await api.deleteSlideshow(show.id);
    if (current?.id === show.id) clear();
    reload();
  };
  const removeShow = async () => {
    if (!current || current.id <= 0 || selectedId === 0) return alertDialog(t('No slideshow selected'), t('No slideshow has been selected to delete.'), t('Please select a slideshow to delete.'));
    await deleteShow(current);
  };

  // drag to re-order the slide shows: the rows slide aside to show the drop position, hit-tested against the rows'
  // resting positions so the sliding can't feed back into it (same as the Schedule list)
  const rowRefs = useRef<(HTMLDivElement | null)[]>([]);
  const slots = useRef<{ top: number; height: number }[]>([]);
  const [dragFrom, setDragFrom] = useState<number | null>(null);
  const [overIdx, setOverIdx] = useState<number | null>(null);
  const slotAt = (y: number) => {
    const list = slots.current;
    if (!list.length) return 0;
    if (y < list[0].top) return 0;
    const k = list.findIndex((sl) => y >= sl.top && y < sl.top + sl.height);
    return k < 0 ? list.length - 1 : k;
  };
  const movedOrder = (from: number, target: number) => {
    const o = ordered.map((_, i) => i).filter((i) => i !== from);
    o.splice(target, 0, from);
    return o;
  };
  const slideOffset = (i: number) => {
    if (dragFrom === null || overIdx === null || dragFrom === overIdx) return 0;
    let y = 0;
    for (const k of movedOrder(dragFrom, overIdx)) {
      if (k === i) break;
      y += slots.current[k]?.height ?? 0;
    }
    return y - slots.current.slice(0, i).reduce((sum, sl) => sum + sl.height, 0);
  };
  const endShowDrag = () => { setDragFrom(null); setOverIdx(null); };
  const dropShow = (y: number) => {
    const from = dragFrom;
    const to = slotAt(y);
    endShowDrag();
    if (from === null || from === to) return;
    const ids = movedOrder(from, to).map((i) => ordered[i].id);
    api.patchApp({ slideshowOrder: ids }).then(() => reloadSettings());
  };

  // ---- photos -----------------------------------------------------------------------------------------------
  // a change to the photos of a saved slide show is stored at once (the images themselves live in the database)
  const saveSlides = async (next: Slide[], keep?: number[]) => {
    setSlides(next);
    setPicked(keep ?? picked.filter((i) => i < next.length));
    if (!current || current.id <= 0 || fromSchedule.current) return;
    await api.saveSlideshow({ ...current, slides: next });
    const ss = await api.slideshow(current.id); // new photos now have their ids
    setCurrent(ss);
    setSlides(ss.slides);
  };
  const addImages = async () => {
    if (!current) return;
    const paths = await pickFiles(boot.desktop, t('Select Images to Open'), [imageFilter], true);
    if (!paths.length) return;
    const added = await api.addImages(paths);
    const next = [...slides, ...added];
    await saveSlides(next, added.map((_, i) => slides.length + i));
    anchor.current = slides.length;
  };
  const addFolder = async () => {
    if (!current) return;
    const f = await pickFolder(boot.desktop, t('Select Folder with Images'));
    if (!f) return;
    const added = await folderSlides(f);
    if (!added.length) return alertDialog(t('No images found'), t('The selected folder has no images.'), t('Please select another folder.'));
    await saveSlides([...slides, ...added], added.map((_, i) => slides.length + i));
    anchor.current = slides.length;
  };
  const removePhotos = (idxs: number[]) => {
    if (!idxs.length) return;
    const gone = new Set(idxs);
    const next = slides.filter((_, i) => !gone.has(i));
    const first = Math.min(...idxs);
    saveSlides(next, next.length ? [Math.min(first, next.length - 1)] : []);
  };
  const onPhotoClick = (i: number, e: React.MouseEvent) => {
    if (e.shiftKey) {
      const lo = Math.min(anchor.current, i);
      const hi = Math.max(anchor.current, i);
      setPicked(Array.from({ length: hi - lo + 1 }, (_, k) => lo + k));
    } else if (e.metaKey || e.ctrlKey) {
      setPicked((p) => (p.includes(i) ? p.filter((x) => x !== i) : [...p, i].sort((a, b) => a - b)));
      anchor.current = i;
    } else {
      setPicked([i]);
      anchor.current = i;
    }
  };

  // drag to re-order the photos, one at a time or the selected batch. The other photos slide to where they will end
  // up; the drop position is worked out from where the photos rested when the drag began, so the sliding can't feed
  // back into it.
  const [dragPhotos, setDragPhotos] = useState<number[] | null>(null);
  const [dropAt, setDropAt] = useState<number | null>(null);
  const gridRef = useRef<HTMLDivElement>(null);
  const thumbRefs = useRef<(HTMLDivElement | null)[]>([]);
  const rest = useRef<{ left: number; top: number; width: number; height: number }[]>([]);
  const captureSlots = () => {
    const g = gridRef.current!.getBoundingClientRect();
    const sl = gridRef.current!;
    rest.current = slides.map((_, k) => {
      const r = thumbRefs.current[k]!.getBoundingClientRect();
      return { left: r.left - g.left + sl.scrollLeft, top: r.top - g.top + sl.scrollTop, width: r.width, height: r.height };
    });
  };
  // the photos in their new order, as positions in the old one
  const previewOrder = (moving: number[], at: number) => {
    const set = new Set(moving);
    const before = moving.filter((k) => k < at).length;
    const others = slides.map((_, i) => i).filter((i) => !set.has(i));
    const pos = at - before;
    return { order: [...others.slice(0, pos), ...moving, ...others.slice(pos)], pos };
  };
  const dropIndexAt = (clientX: number, clientY: number) => {
    const list = rest.current;
    if (!list.length || !gridRef.current) return 0;
    const g = gridRef.current.getBoundingClientRect();
    const x = clientX - g.left + gridRef.current.scrollLeft;
    const y = clientY - g.top + gridRef.current.scrollTop;
    let best = 0;
    let bestD = Infinity;
    list.forEach((r, k) => {
      const dx = x < r.left ? r.left - x : x > r.left + r.width ? x - (r.left + r.width) : 0;
      const dy = y < r.top ? r.top - y : y > r.top + r.height ? y - (r.top + r.height) : 0;
      const d = dx * dx + dy * dy;
      if (d < bestD) { bestD = d; best = k; }
    });
    const r = list[best];
    return x > r.left + r.width / 2 ? best + 1 : best;
  };
  const photoOffset = (k: number) => {
    if (!dragPhotos || dropAt === null || !rest.current.length) return undefined;
    const p = previewOrder(dragPhotos, dropAt).order.indexOf(k);
    const to = rest.current[p];
    const from = rest.current[k];
    if (!to || !from) return undefined;
    return `translate(${to.left - from.left}px, ${to.top - from.top}px)`;
  };
  const endPhotoDrag = () => { setDragPhotos(null); setDropAt(null); };
  const dropPhotos = (at: number) => {
    const moving = dragPhotos;
    endPhotoDrag();
    if (!moving || !moving.length) return;
    const { order, pos } = previewOrder(moving, at);
    const next = order.map((k) => slides[k]);
    if (next.every((s, i) => s === slides[i])) return;
    saveSlides(next, moving.map((_, k) => pos + k));
    anchor.current = pos;
  };

  const idx = picked.length ? picked[picked.length - 1] : 0;
  const currentFull = (): SlideShowFull => ({ ...(current || { id: 0, name: '', info: '' }), slides });
  const goLive = () => {
    if (!slides.length) return;
    api.showSlideshow(currentFull(), Math.min(idx, slides.length - 1));
    focusLive();
  };
  const addToSchedule = () => {
    if (!slides.length) return;
    const ss = currentFull();
    const item: ScheduleItem = { type: 'slideshow', name: ss.name, slideshow: ss };
    api.scheduleAdd(item);
  };
  useRegisterPanel('pictures', {
    newItem: () => setCreating(true),
    edit: () => (current && selectedId ? ui.openDialog({ kind: 'slideshow', slideshow: currentFull() }) : alertDialog(t('No slideshow selected'), t('No slideshow has been selected to edit.'), t('Please select a slideshow to edit.'))),
    remove: removeShow,
    goLive,
    focusBrowser: () => gridRef.current?.focus(),
    addToSchedule,
    hasSelection: () => slides.length > 0,
    showFromSchedule: (item) => {
      const it = item as ScheduleItem;
      if (it.slideshow) {
        fromSchedule.current = true;
        setSelectedId(0);
        setCurrent(it.slideshow);
        setSlides(it.slideshow.slides);
        setPicked(it.slideshow.slides.length ? [0] : []);
        anchor.current = 0;
      }
    },
  });

  return (
    <>
      <Splitter id="pictures-main" initial={0.3} saved={settings.app.splitters} className="fill">
        <div className="col fill" style={{ gap: 4 }}>
          <div className="row" style={{ gap: 2 }}>
            <span style={{ fontWeight: 600 }}>{t('Slide Shows:')}</span>
            <span className="grow" />
            <button className="tbtn-s" title={t('New Slide Show from Folder...')} disabled={picking} onClick={() => setCreatingFolder(true)}><FolderPlusIcon /></button>
            <button className="tbtn-s" title={t('New Slide Show...')} onClick={() => setCreating(true)}><PlusIcon /></button>
            <button className="tbtn-s" title={t('Delete Slide Show')} disabled={!selectedId} onClick={removeShow}><MinusIcon /></button>
          </div>
          <div className="show-head"><span>{t('Title')}</span><span>{t('Added')}</span></div>
          <div
            className={`list grow show-list ${active ? 'active' : ''}`}
            tabIndex={0}
            onDragOver={(e) => {
              if (dragFrom === null) return;
              e.preventDefault();
              const target = slotAt(e.clientY);
              setOverIdx((o) => (o === target ? o : target));
            }}
            onDrop={(e) => { e.preventDefault(); dropShow(e.clientY); }}
          >
            {ordered.map((s, i) => (
              <div
                key={s.id}
                ref={(el) => { rowRefs.current[i] = el; }}
                draggable
                onDragStart={(e) => {
                  // dropped on the Schedule it is added there (photos are fetched on drop), in the list it re-orders
                  startScheduleDrag(e, { type: 'slideshow', name: s.name, slideshow: { ...s, slides: [] } });
                  e.dataTransfer.effectAllowed = 'copyMove';
                  slots.current = ordered.map((_, k) => {
                    const r = rowRefs.current[k]!.getBoundingClientRect();
                    return { top: r.top, height: r.height };
                  });
                  setTimeout(() => { setDragFrom(i); setOverIdx(i); }, 0);
                }}
                onDragEnd={endShowDrag}
                className={`item ${s.id === selectedId ? 'sel' : ''}`}
                style={{
                  transform: dragFrom !== null ? `translateY(${slideOffset(i)}px)` : undefined,
                  opacity: dragFrom === i ? 0.45 : 1,
                  transition: dragFrom !== null ? 'transform 0.15s ease' : undefined,
                  cursor: 'grab',
                }}
                onClick={() => select(s)}
                onDoubleClick={() => { select(s); api.slideshow(s.id).then((ss) => { if (ss.slides.length) { api.showSlideshow(ss, 0); focusLive(); } }); }}
              >
                <span className="show-name">{s.name}</span>
                <span className="show-date">{s.created}</span>
                <button className="sched-del show-del" title={t('Delete Slide Show')} aria-label={t('Delete Slide Show')} draggable={false}
                  onMouseDown={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}
                  onClick={(e) => { e.stopPropagation(); deleteShow(s); }}><TrashIcon /></button>
                <button className="sched-del show-edit" title={t('Rename')} aria-label={t('Rename')} draggable={false}
                  onMouseDown={(e) => e.stopPropagation()} onDoubleClick={(e) => e.stopPropagation()}
                  onClick={(e) => { e.stopPropagation(); setRenaming(s); }}><PencilIcon /></button>
              </div>
            ))}
          </div>
        </div>
        <div className="col fill" style={{ gap: 4 }}>
          <div className="row" style={{ gap: 2 }}>
            <span style={{ fontWeight: 600, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap' }}>{current && current.name ? current.name : ''}</span>
            <span className="grow" />
            <button className="tbtn-s" title={t('Add Images from Folder...')} disabled={!current || picking} onClick={addFolder}><FolderPlusIcon /></button>
            <button className="tbtn-s" title={t('Add Images...')} disabled={!current || picking} onClick={addImages}><PlusIcon /></button>
            <button className="tbtn-s" title={t('Remove Image')} disabled={!picked.length} onClick={() => removePhotos(picked)}><MinusIcon /></button>
            <button style={{ marginLeft: 14 }} onClick={() => setViewing(true)} disabled={!slides.length} title={t('Preview (Space)')}>{t('Preview')}</button>
            <button className="primary golive" style={{ marginLeft: 4 }} onClick={goLive} disabled={!slides.length} title={t('Go Live (F5)')} aria-label={t('Go Live (F5)')}><img className="icon" src="/icons/go_live.png" alt="" /></button>
          </div>
          <div
            ref={gridRef}
            className="list grow thumb-list"
            tabIndex={0}
            onKeyDown={(e) => {
              if (e.key === 'ArrowRight' || e.key === 'ArrowDown') { const n = Math.min(slides.length - 1, idx + 1); setPicked([n]); anchor.current = n; e.preventDefault(); e.stopPropagation(); }
              else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') { const n = Math.max(0, idx - 1); setPicked([n]); anchor.current = n; e.preventDefault(); e.stopPropagation(); }
              else if (e.key === ' ' && slides.length) { setViewing(true); e.preventDefault(); e.stopPropagation(); }
              else if (e.key === 'Enter') goLive();
              else if (e.key === 'Delete' || e.key === 'Backspace') { removePhotos(picked); e.preventDefault(); e.stopPropagation(); }
              else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'a') { setPicked(slides.map((_, i) => i)); e.preventDefault(); }
            }}
            onDragOver={(e) => { if (dragPhotos) { e.preventDefault(); const at = dropIndexAt(e.clientX, e.clientY); setDropAt((o) => (o === at ? o : at)); } }}
            onDrop={(e) => { e.preventDefault(); dropPhotos(dropIndexAt(e.clientX, e.clientY)); }}
          >
            {slides.map((s, i) => (
              <div
                key={s.id || s.temp}
                ref={(el) => { thumbRefs.current[i] = el; }}
                className={`thumb ${picked.includes(i) ? 'sel' : ''} ${dragPhotos?.includes(i) ? 'dragging' : ''}`}
                style={{ transform: photoOffset(i), transition: dragPhotos ? 'transform 0.15s ease' : undefined, zIndex: dragPhotos?.includes(i) ? 1 : undefined }}
                draggable
                onDragStart={(e) => {
                  const moving = picked.includes(i) ? picked : [i];
                  if (!picked.includes(i)) { setPicked([i]); anchor.current = i; }
                  e.dataTransfer.effectAllowed = 'move';
                  e.dataTransfer.setData('text/plain', moving.join(','));
                  captureSlots();
                  setTimeout(() => { setDragPhotos(moving); setDropAt(i); }, 0);
                }}
                onDragEnd={endPhotoDrag}
                onClick={(e) => onPhotoClick(i, e)}
                onDoubleClick={() => { setPicked([i]); anchor.current = i; api.showSlideshow(currentFull(), i); focusLive(); }}
                title={s.name}
              >
                <img src={slideURL(s, 'small')} alt={s.name} draggable={false} />
                <div className="thumb-actions">
                  <button title={t('Delete')} aria-label={t('Delete')} onClick={(e) => { e.stopPropagation(); removePhotos(picked.includes(i) && picked.length > 1 ? picked : [i]); }}><TrashIcon /></button>
                </div>
              </div>
            ))}
          </div>
        </div>
      </Splitter>
      {viewing && slides.length > 0 && <PhotoViewer slides={slides} index={Math.min(idx, slides.length - 1)} url={slideURL} onIndex={(n) => { setPicked([n]); anchor.current = n; }} onClose={() => { setViewing(false); gridRef.current?.focus(); }} />}
      {creatingFolder && <FolderShowDialog onCreate={createShowFromFolder} onClose={() => setCreatingFolder(false)} />}
      {renaming && <NameInfoDialog title={t('Rename Slide Show')} nameLabel={t('Title:')} name={renaming.name} onSave={(n) => renameShow(n)} onClose={() => setRenaming(null)} />}
      {creating && <NameInfoDialog title={t('New Slide Show')} nameLabel={t('Title:')} infoLabel={t('Information:')} onSave={createShow} onClose={() => setCreating(false)} />}
    </>
  );
}
