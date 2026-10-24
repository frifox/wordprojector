import { useEffect, useRef, useState } from 'react';
import type { Slide } from '../lib/types';
import { t } from '../i18n';

function sizeText(bytes: number): string {
  if (bytes >= 1024 * 1024) return `${(bytes / 1024 / 1024).toFixed(1)} MB`;
  if (bytes >= 1024) return `${Math.round(bytes / 1024)} KB`;
  return `${bytes} B`;
}

/** Full window view of one photo with a thumbnail strip: arrows move, Space or Esc close. */
export default function PhotoViewer({ slides, index, url, onIndex, onClose, onUse }: {
  slides: Slide[]; index: number; url: (s: Slide, size: 'full' | 'small' | 'prev') => string;
  onIndex: (i: number) => void; onClose: () => void;
  /** adds a "Use This" button at the bottom right: it takes the picture on view and closes the viewer */
  onUse?: (i: number) => void;
}) {
  const strip = useRef<HTMLDivElement>(null);
  const state = useRef({ index, count: slides.length, onIndex, onClose, onUse });
  state.current = { index, count: slides.length, onIndex, onClose, onUse };

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const { index: i, count, onIndex: go, onClose: close, onUse: use } = state.current;
      let handled = true;
      if (e.key === 'ArrowRight' || e.key === 'ArrowDown') go(Math.min(count - 1, i + 1));
      else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') go(Math.max(0, i - 1));
      else if (e.key === 'Home') go(0);
      else if (e.key === 'End') go(count - 1);
      else if (e.key === ' ' || e.key === 'Escape') close();
      // Enter does what the "use" button does (when the viewer has one)
      else if (e.key === 'Enter' && use) { use(i); close(); }
      else handled = false;
      if (handled) e.preventDefault();
      // keep the shortcuts of the operator window (typing, Esc clearing the screen) from acting behind the viewer
      if (handled || !(/^F\d+$/.test(e.key) || e.metaKey || e.ctrlKey)) {
        e.stopPropagation();
        // a dialog under the viewer must not see the key either (it would close on the Esc that closes the viewer)
        e.stopImmediatePropagation();
      }
    };
    window.addEventListener('keydown', h, true);
    return () => window.removeEventListener('keydown', h, true);
  }, []);

  useEffect(() => {
    const el = strip.current?.children[index] as HTMLElement | undefined;
    el?.scrollIntoView({ block: 'nearest', inline: 'center' });
  }, [index]);

  const cur = slides[index];
  // the resolution comes from the picture once it is loaded, the size of the file from the server's answer about it
  const [natural, setNatural] = useState<{ id: string; w: number; h: number } | null>(null);
  const [bytes, setBytes] = useState<{ id: string; n: number } | null>(null);
  const curUrl = cur ? url(cur, 'full') : '';
  useEffect(() => {
    if (!curUrl) return;
    let live = true;
    fetch(curUrl, { method: 'HEAD' }).then((r) => {
      const n = Number(r.headers.get('content-length'));
      if (live && n > 0) setBytes({ id: curUrl, n });
    }).catch(() => {});
    return () => { live = false; };
  }, [curUrl]);
  if (!cur) return null;
  const parts: string[] = [];
  if (natural?.id === curUrl) parts.push(`${natural.w} × ${natural.h}`);
  if (bytes?.id === curUrl) parts.push(sizeText(bytes.n));
  return (
    <div className="photo-viewer" onMouseDown={(e) => e.stopPropagation()}>
      <div className="pv-top">
        {/* resolution, size, name: a line between each */}
        {parts.map((p, i) => (
          <span key={p} className="pv-piece">{i > 0 && <span className="pv-sep" aria-hidden="true" />}<span className="pv-info">{p}</span></span>
        ))}
        <span className="pv-piece" style={{ flex: '0 1 auto', minWidth: 0 }}>{parts.length > 0 && <span className="pv-sep" aria-hidden="true" />}<span className="pv-name">{cur.name}</span></span>
        <span style={{ flex: 1 }} />
        <span className="pv-count">{index + 1} / {slides.length}</span>
        <button title={t('Close')} aria-label={t('Close')} onClick={onClose}>✕</button>
      </div>
      <div className="pv-main" onClick={onClose}>
        <img src={curUrl} alt={cur.name} draggable={false} onClick={(e) => e.stopPropagation()}
          onLoad={(e) => setNatural({ id: curUrl, w: e.currentTarget.naturalWidth, h: e.currentTarget.naturalHeight })} />
      </div>
      <div className="pv-bottom">
        <div className="pv-strip" ref={strip}>
          {slides.map((s, i) => (
            <div key={s.id || s.temp} className={`pv-thumb ${i === index ? 'sel' : ''}`} onClick={() => onIndex(i)} title={s.name}>
              <img src={url(s, 'small')} alt={s.name} draggable={false} />
            </div>
          ))}
        </div>
        {onUse && (
          <button className="pv-use" onClick={() => { onUse(index); onClose(); }}>
            {t('Use as background')}
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M5 12h14" /><path d="M13 18l6 -6" /><path d="M13 6l6 6" /></svg>
          </button>
        )}
      </div>
    </div>
  );
}
