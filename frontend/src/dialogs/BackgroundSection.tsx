import { useEffect, useState } from 'react';
import { alertDialog } from '../components/Modal';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { PassiveSettings, ThemeImage } from '../lib/types';
import { pickFiles, imageFilter } from '../lib/files';

/** One image in the collection. While the picture is still being added or loaded it shows a spinner instead. */
function BgTile({ src, name, selected, aspect, onChoose, onRemove, onRename }: { src: string | null; name: string; selected: boolean; aspect: number; onChoose?: () => void; onRemove?: () => void; onRename?: (name: string) => void }) {
  const [loaded, setLoaded] = useState(false);
  const [editing, setEditing] = useState<string | null>(null); // the new name while it is being typed
  const finishRename = (save: boolean) => {
    const v = (editing ?? '').trim();
    setEditing(null);
    if (save && v && v !== name) onRename?.(v);
  };
  const ready = !!src && loaded;
  return (
    <div style={{ position: 'relative' }} className="bg-tile">
      <button className={`ui-thumb ${selected ? 'on' : ''}`} onClick={onChoose} disabled={!ready} aria-pressed={selected} aria-busy={!ready} title={name}>
        <div style={{ width: 120, aspectRatio: String(aspect), background: '#000', border: '1px solid var(--border)', borderRadius: 3, overflow: 'hidden', position: 'relative' }}>
          {src && <img src={src} alt="" draggable={false} onLoad={() => setLoaded(true)} onError={() => setLoaded(true)}
            ref={(el) => { if (el && el.complete && el.naturalWidth > 0 && !loaded) setLoaded(true); }}
            style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'cover', opacity: ready ? 1 : 0 }} />}
          {!ready && <div className="bg-spinner-wrap"><div className="bg-spinner" /></div>}
        </div>
        {editing === null && <span style={{ maxWidth: 120, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', fontSize: 12 }}>{name}</span>}
      </button>
      {editing !== null && (
        <input data-esc-own autoFocus value={editing} onChange={(e) => setEditing(e.target.value)} onFocus={(e) => e.currentTarget.select()} onBlur={() => finishRename(true)}
          onKeyDown={(e) => { if (e.key === 'Enter') finishRename(true); else if (e.key === 'Escape') { e.stopPropagation(); finishRename(false); } }}
          style={{ position: 'absolute', left: 6, right: 6, bottom: 6, width: 'auto', fontSize: 12, padding: '1px 4px' }} />
      )}
      {ready && editing === null && (onRename || onRemove) && (
        <div className="bg-actions">
          {onRename && (
            <button title={t('Rename')} aria-label={t('Rename')} onClick={() => setEditing(name)}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 20h4l10.5 -10.5a2.828 2.828 0 1 0 -4 -4l-10.5 10.5v4" /><path d="M13.5 6.5l4 4" /></svg>
            </button>
          )}
          {onRemove && (
            <button title={t('Remove')} aria-label={t('Remove')} onClick={onRemove}>
              <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M4 7h16" /><path d="M10 11v6" /><path d="M14 11v6" /><path d="M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2 -2l1 -12" /><path d="M9 7v-3a1 1 0 0 1 1 -1h4a1 1 0 0 1 1 1v3" /></svg>
            </button>
          )}
        </div>
      )}
    </div>
  );
}

/** Little pictures of the three ways an image can fill the screen (the screen is the outline). */
function ScaleIcon({ mode }: { mode: number }) {
  const img = 'var(--accent)';
  return (
    <svg viewBox="0 0 56 44" width={56} height={44} aria-hidden="true" style={{ display: 'block', color: 'var(--text)' }}>
      <defs><clipPath id={`screen-${mode}`}><rect x="4" y="9" width="48" height="26" /></clipPath></defs>
      {mode === 1 && (
        <>
          {/* Fill: the image is bigger than the screen, so its top and bottom are cut off */}
          <rect x="4" y="1" width="48" height="42" fill="none" stroke="currentColor" strokeWidth="1" strokeDasharray="2 2" opacity="0.6" />
          <rect x="4" y="1" width="48" height="42" fill={img} clipPath={`url(#screen-${mode})`} />
        </>
      )}
      {mode === 2 && (
        <>
          {/* Fit: the whole image, with bars where the proportions differ */}
          <rect x="4" y="9" width="48" height="26" fill="#000" />
          <rect x="15" y="9" width="26" height="26" fill={img} />
        </>
      )}
      {mode === 0 && (
        <>
          {/* Scale: stretched edge to edge in both directions */}
          <rect x="4" y="9" width="48" height="26" fill={img} />
          <g stroke="#fff" strokeWidth="1.6" strokeLinecap="round" strokeLinejoin="round" fill="none">
            <path d="M12 22h32M12 22l4 -3M12 22l4 3M44 22l-4 -3M44 22l-4 3" />
            <path d="M28 14v16M28 14l-3 4M28 14l3 4M28 30l-3 -4M28 30l3 -4" />
          </g>
        </>
      )}
      <rect x="4" y="9" width="48" height="26" fill="none" stroke="currentColor" strokeWidth="1.5" />
    </svg>
  );
}

/**
 * The picture of the idle screen of a theme: the pool of pictures the theme has (added, renamed and removed at once), the
 * one chosen from it (kept with the theme when it is saved), and how it fills the screen.
 */
export default function BackgroundSection({ themeId, v, onChange, onPool }: { themeId: number; v: PassiveSettings; onChange: (p: Partial<PassiveSettings>) => void; onPool: (ids: number[]) => void }) {
  const { boot } = useStore();
  // the pictures this theme keeps
  const [images, setImages] = useState<ThemeImage[]>([]);
  const [pending, setPending] = useState<string[]>([]);
  const selected = images.find((i) => i.id === v.imageId);
  // after the pool was changed the editor looks at what the server chose (the first picture is chosen at once, and so on)
  const apply = (list: ThemeImage[]) => {
    setImages(list);
    onPool(list.map((i) => i.id));
  };
  useEffect(() => {
    api.themeImages(themeId, 'passive', 1).then(setImages).catch(() => setImages([]));
  }, [themeId]);
  // pixel size of the selected image, for the size indicator
  useEffect(() => {
    if (!selected) { setNatural(null); return; }
    const im = new Image();
    im.onload = () => setNatural({ w: im.naturalWidth, h: im.naturalHeight });
    im.src = `/api/theme-images/${selected.id}`;
  }, [selected?.id]); // eslint-disable-line react-hooks/exhaustive-deps
  const add = async () => {
    const paths = await pickFiles(boot.desktop, t('Select images for the wallpaper'), [imageFilter], true);
    if (!paths.length) return;
    setPending(paths.map((p) => p.split(/[\\/]/).pop() || p));
    try { apply(await api.addThemeImages(themeId, 'passive', 1, paths)); } catch (e) { alertDialog(t('Error'), String((e as Error).message || e)); } finally { setPending([]); }
  };
  const choose = (img: ThemeImage) => onChange({ imageId: img.id, useBackground: true });
  const renameImg = async (img: ThemeImage, name: string) => {
    try { apply(await api.renameThemeImage(themeId, 'passive', 1, img.id, name)); } catch (e) { alertDialog(t('Error'), String((e as Error).message || e)); }
  };
  const removeImg = async (img: ThemeImage) => { apply(await api.removeThemeImage(themeId, 'passive', 1, img.id)); };
  const fit = v.backgroundFit ?? 0; // 0 scale, 1 fill, 2 fit (the tiles show Fill, Fit, Scale)
  // size of the picture and of the screen it is shown on, for the "% of original" indicator
  const [natural, setNatural] = useState<{ w: number; h: number } | null>(null);
  const { live } = useStore();
  const disp = live.displays[0];
  const screenW = disp?.width || 1920;
  const screenH = disp?.height || 1080;
  let info: { image: string; screen: string; shown: string } | null = null;
  if (natural && selected) {
    const sx = screenW / natural.w;
    const sy = screenH / natural.h;
    const pct = (x: number) => `${Math.round(x * 100)}%`;
    const shown = fit === 1 ? pct(Math.max(sx, sy)) : fit === 2 ? pct(Math.min(sx, sy)) : Math.abs(sx - sy) < 0.005 ? pct(sx) : `${pct(sx)} × ${pct(sy)}`;
    info = { image: `${natural.w} × ${natural.h} px`, screen: `${screenW} × ${screenH} px`, shown };
  }
  return (
    <fieldset className="group">
      <legend><label className="chk"><input type="checkbox" checked={v.useBackground} onChange={(e) => onChange({ useBackground: e.target.checked })} />{t('Use Background Image')}</label></legend>
      {/* the collection: its own framed box with a header bar and a scrolling area of thumbnails */}
      <div style={{ border: '1px solid var(--border)', borderRadius: 4, background: 'var(--base)', display: 'flex', flexDirection: 'column', overflow: 'hidden' }}>
        <div className="row" style={{ padding: '4px 8px', background: 'var(--panel)', borderBottom: '1px solid var(--border)' }}>
          <b style={{ fontSize: 12 }}>{t('Images')}</b>
          <span className="muted" style={{ fontSize: 12 }}>{images.length}</span>
          <span className="grow" />
          <button className="small" onClick={add} title={t('Add images...')}>{t('Add...')}</button>
        </div>
        <div className="row" style={{ gap: 10, alignItems: 'flex-start', flexWrap: 'wrap', padding: 8, minHeight: 90, maxHeight: 260, overflow: 'auto' }}>
          {images.map((img) => (
            <BgTile key={img.id} src={`/api/theme-images/${img.id}`} name={img.name} selected={img.id === v.imageId} aspect={screenW / screenH} onChoose={() => choose(img)} onRemove={() => removeImg(img)} onRename={(n) => renameImg(img, n)} />
          ))}
          {/* files being added: placeholders with a spinner until the server has them ready */}
          {pending.map((name, i) => <BgTile key={`pending-${i}-${name}`} src={null} name={name} selected={false} aspect={screenW / screenH} />)}
          {images.length === 0 && pending.length === 0 && <div className="muted" style={{ alignSelf: 'center', width: '100%', textAlign: 'center' }}>{t('No images yet. Use Add... to choose some.')}</div>}
        </div>
      </div>
      {(
        <div className="col" style={{ marginTop: 6, gap: 4 }}>
          <div className="row" style={{ gap: 10, alignItems: 'flex-start', flexWrap: 'wrap' }} role="radiogroup" aria-label={t('Image scaling:')}>
            {([[1, t('Fill')], [2, t('Fit')], [0, t('Scale')]] as const).map(([value, label]) => (
              <button key={value} className={`ui-thumb ${fit === value ? 'on' : ''}`} disabled={!selected} role="radio" aria-checked={fit === value}
                onClick={() => onChange({ backgroundFit: value })}
                title={value === 1 ? t('Fills the whole screen; the edges of the image may be cut off.') : value === 2 ? t('Shows the whole image; black bars appear when its proportions differ from the screen.') : t('Stretches the image edge to edge and ignores its proportions.')}>
                <ScaleIcon mode={value} />
                <span>{label}</span>
              </button>
            ))}
          </div>
          {info && (
            <div className="form-grid" style={{ gridTemplateColumns: 'auto auto', justifyContent: 'start', columnGap: 12, rowGap: 2, fontSize: 12 }}>
              <span className="muted">{t('Image size:')}</span><span>{info.image}</span>
              <span className="muted">{t('Screen size:')}</span><span>{info.screen}</span>
              <span className="muted">{t('Shown at:')}</span><span style={{ fontWeight: 600 }}>{info.shown}</span>
            </div>
          )}
        </div>
      )}
    </fieldset>
  );
}

