import { useEffect, useRef } from 'react';
import type { Slide } from '../lib/types';
import { t } from '../i18n';

/** Full window view of one photo with a thumbnail strip: arrows move, Space or Esc close. */
export default function PhotoViewer({ slides, index, url, onIndex, onClose }: {
  slides: Slide[]; index: number; url: (s: Slide, size: 'full' | 'small' | 'prev') => string;
  onIndex: (i: number) => void; onClose: () => void;
}) {
  const strip = useRef<HTMLDivElement>(null);
  const state = useRef({ index, count: slides.length, onIndex, onClose });
  state.current = { index, count: slides.length, onIndex, onClose };

  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      const { index: i, count, onIndex: go, onClose: close } = state.current;
      let handled = true;
      if (e.key === 'ArrowRight' || e.key === 'ArrowDown') go(Math.min(count - 1, i + 1));
      else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') go(Math.max(0, i - 1));
      else if (e.key === 'Home') go(0);
      else if (e.key === 'End') go(count - 1);
      else if (e.key === ' ' || e.key === 'Escape') close();
      else handled = false;
      if (handled) e.preventDefault();
      // keep the shortcuts of the operator window (typing, Esc clearing the screen) from acting behind the viewer
      if (handled || !(/^F\d+$/.test(e.key) || e.metaKey || e.ctrlKey)) e.stopPropagation();
    };
    window.addEventListener('keydown', h, true);
    return () => window.removeEventListener('keydown', h, true);
  }, []);

  useEffect(() => {
    const el = strip.current?.children[index] as HTMLElement | undefined;
    el?.scrollIntoView({ block: 'nearest', inline: 'center' });
  }, [index]);

  const cur = slides[index];
  if (!cur) return null;
  return (
    <div className="photo-viewer" onMouseDown={(e) => e.stopPropagation()}>
      <div className="pv-top">
        <span className="pv-name">{cur.name}</span>
        <span className="pv-count">{index + 1} / {slides.length}</span>
        <button title={t('Close')} aria-label={t('Close')} onClick={onClose}>✕</button>
      </div>
      <div className="pv-main" onClick={onClose}>
        <img src={url(cur, 'full')} alt={cur.name} draggable={false} onClick={(e) => e.stopPropagation()} />
      </div>
      <div className="pv-strip" ref={strip}>
        {slides.map((s, i) => (
          <div key={s.id || s.temp} className={`pv-thumb ${i === index ? 'sel' : ''}`} onClick={() => onIndex(i)} title={s.name}>
            <img src={url(s, 'small')} alt={s.name} draggable={false} />
          </div>
        ))}
      </div>
    </div>
  );
}
