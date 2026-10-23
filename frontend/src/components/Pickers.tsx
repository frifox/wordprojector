import { useEffect, useLayoutEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import type { Font } from '../lib/types';
import { api } from '../lib/api';
import { t } from '../i18n';

let fontsPromise: Promise<string[]> | null = null;
export function loadFonts(): Promise<string[]> {
  if (!fontsPromise) fontsPromise = api.fonts().catch(() => ['Arial']);
  return fontsPromise;
}

export function useFonts(): string[] {
  const [fonts, setFonts] = useState<string[]>(['Arial']);
  useEffect(() => {
    loadFonts().then(setFonts);
  }, []);
  return fonts;
}

export function fontLabel(f: Font): string {
  return `${f.family} ${f.pointSize}${f.bold ? ' ' + t('Bold') : ''}${f.italic ? ' ' + t('Italic') : ''}`;
}

/** Bold and italic toggle buttons. */
export function FontStyleButtons({ value, onChange, disabled }: { value: Font; onChange: (f: Font) => void; disabled?: boolean }) {
  return (
    <span className="btn-group" role="group">
        {([['bold', t('Bold'), <><path d="M7 5h6a3.5 3.5 0 0 1 0 7h-6z" /><path d="M13 12h1a3.5 3.5 0 0 1 0 7h-7v-7" /></>],
          ['italic', t('Italic'), <><path d="M11 5l6 0" /><path d="M7 19l6 0" /><path d="M14 5l-4 14" /></>]] as const).map(([key, label, icon]) => (
          <button key={key} type="button" className={value[key] ? 'on' : ''} title={label} aria-label={label} aria-pressed={value[key]} disabled={disabled}
            onClick={() => onChange({ ...value, [key]: !value[key] })} style={{ padding: '3px 7px' }}>
            <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{icon}</svg>
          </button>
        ))}
      </span>
  );
}

/**
 * A font list that shows every family in its own face. While it is open, the font under the pointer (or the arrow keys)
 * is reported through onPreview, and null when it closes, so a preview can show it before it is chosen.
 */
function FontSelect({ value, fonts, disabled, onChange, onPreview }: { value: string; fonts: string[]; disabled?: boolean; onChange: (family: string) => void; onPreview?: (family: string | null) => void }) {
  const [open, setOpen] = useState(false);
  const [rect, setRect] = useState<DOMRect | null>(null);
  const [cur, setCur] = useState(value); // the highlighted family
  const btn = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const close = () => { setOpen(false); onPreview?.(null); };
  const choose = (f: string) => { setOpen(false); onPreview?.(null); onChange(f); btn.current?.focus(); };
  const show = (f: string) => { setCur(f); onPreview?.(f === value ? null : f); };
  useLayoutEffect(() => {
    if (!open) return;
    setRect(btn.current?.getBoundingClientRect() ?? null);
    list.current?.querySelector<HTMLElement>('[data-current="true"]')?.scrollIntoView({ block: 'center' });
  }, [open]);
  useEffect(() => {
    if (!open) return;
    const away = (e: MouseEvent) => { if (!list.current?.contains(e.target as Node) && !btn.current?.contains(e.target as Node)) close(); };
    window.addEventListener('mousedown', away, true);
    return () => window.removeEventListener('mousedown', away, true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);
  // type-ahead, like jumping to a file by typing its name: the letters typed in quick succession are matched against the
  // start of the names; after a pause the next letter starts a new search
  const typed = useRef({ text: '', at: 0 });
  const jump = (ch: string): boolean => {
    const now = Date.now();
    const q = (now - typed.current.at > 900 ? '' : typed.current.text) + ch.toLowerCase();
    typed.current = { text: q, at: now };
    const hit = fonts.find((f) => f.toLowerCase().startsWith(q)) ?? (q.length === 1 ? undefined : fonts.find((f) => f.toLowerCase().includes(q)));
    if (!hit) return false;
    if (!open) { setRect(btn.current?.getBoundingClientRect() ?? null); setOpen(true); }
    show(hit);
    window.setTimeout(() => list.current?.querySelector<HTMLElement>(`[data-font="${CSS.escape(hit)}"]`)?.scrollIntoView({ block: 'center' }), 0);
    return true;
  };
  const key = (e: React.KeyboardEvent) => {
    const printable = e.key.length === 1 && !e.ctrlKey && !e.metaKey && !e.altKey && (e.key !== ' ' || (typed.current.text !== '' && Date.now() - typed.current.at <= 900));
    if (printable) { e.preventDefault(); jump(e.key); return; }
    if (!open) { if (e.key === 'ArrowDown' || e.key === 'ArrowUp' || e.key === 'Enter' || e.key === ' ') { e.preventDefault(); setCur(value); setOpen(true); } return; }
    const at = fonts.indexOf(cur);
    if (e.key === 'ArrowDown' || e.key === 'ArrowUp') {
      e.preventDefault();
      const next = fonts[Math.max(0, Math.min(fonts.length - 1, at + (e.key === 'ArrowDown' ? 1 : -1)))];
      show(next);
      list.current?.querySelector<HTMLElement>(`[data-font="${CSS.escape(next)}"]`)?.scrollIntoView({ block: 'nearest' });
    } else if (e.key === 'Enter') { e.preventDefault(); choose(cur); }
    else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); close(); }
  };
  return (
    <>
      <button ref={btn} type="button" data-esc-own={open ? '' : undefined} disabled={disabled} onClick={() => { setCur(value); setOpen((o) => { if (o) onPreview?.(null); return !o; }); }} onKeyDown={key}
        style={{ width: 170, justifyContent: 'space-between', fontFamily: value, overflow: 'hidden' }}>
        <span style={{ overflow: 'hidden', textOverflow: 'ellipsis' }}>{value}</span>
        <svg viewBox="0 0 24 24" width="12" height="12" fill="none" stroke="currentColor" strokeWidth="2.5" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d="M6 9l6 6l6 -6" /></svg>
      </button>
      {open && rect && createPortal(
        <div ref={list} role="listbox" onMouseLeave={() => { setCur(value); onPreview?.(null); }}
          style={{ position: 'fixed', left: rect.left, top: Math.min(rect.bottom + 2, window.innerHeight - 300), width: Math.max(rect.width, 220), maxHeight: 296, overflowY: 'auto', zIndex: 100000,
            background: 'var(--panel, #2b2b2b)', border: '1px solid var(--border-strong)', borderRadius: 'var(--radius)', boxShadow: '0 6px 18px rgba(0,0,0,0.35)', padding: 2 }}>
          {fonts.map((f) => (
            <div key={f} role="option" aria-selected={f === value} data-font={f} data-current={f === value}
              onMouseEnter={() => show(f)} onClick={() => choose(f)}
              style={{ padding: '3px 8px', fontFamily: f, fontSize: 15, whiteSpace: 'nowrap', overflow: 'hidden', textOverflow: 'ellipsis', borderRadius: 3, cursor: 'default',
                background: f === cur ? 'var(--accent)' : undefined, color: f === cur ? 'var(--accent-text, #fff)' : undefined, outline: f === value && f !== cur ? '1px solid var(--accent)' : undefined, outlineOffset: -1 }}>{f}</div>
          ))}
        </div>, document.body)}
    </>
  );
}

/** Inline font editor: family, size, color, and bold / italic toggle buttons (unless noStyle: place FontStyleButtons elsewhere). */
export function FontPicker({ value, onChange, disabled, color, noStyle, onPreview }: { value: Font; onChange: (f: Font) => void; disabled?: boolean; color?: React.ReactNode; noStyle?: boolean; onPreview?: (f: Font | null) => void }) {
  const fonts = useFonts();
  const list = fonts.includes(value.family) ? fonts : [value.family, ...fonts];
  return (
    <span className="row" style={{ gap: 4 }}>
      <FontSelect value={value.family} fonts={list} disabled={disabled} onChange={(family) => onChange({ ...value, family })} onPreview={onPreview && ((family) => onPreview(family === null ? null : { ...value, family }))} />
      {/* a small and a large serif T side by side, like the font size icon of image editors */}
      <svg viewBox="0 0 24 24" width="20" height="20" fill="none" stroke="currentColor" strokeWidth="1.8" strokeLinecap="square" strokeLinejoin="miter" role="img" aria-label={t('Text size')} style={{ marginLeft: 10, opacity: disabled ? 0.45 : 1 }}>
        <title>{t('Text size')}</title>
        <path d="M11 5h11M11 5v2.5M22 5v2.5M16.5 5v14M14 19h5" />
        <path d="M2 11.5h7.5M2 11.5v2M9.5 11.5v2M5.75 11.5v7.5M3.75 19h4" />
      </svg>
      <input type="number" min={4} max={200} value={value.pointSize} disabled={disabled} style={{ width: 52 }} title={t('Text size. It is relative to the screen height, so the text looks the same on every display and in the preview.')} onChange={(e) => onChange({ ...value, pointSize: Number(e.target.value) || 16 })} />
      {color}
      {!noStyle && <FontStyleButtons value={value} onChange={onChange} disabled={disabled} />}
    </span>
  );
}

/** Color button with a native color input. */
export function ColorField({ value, onChange, disabled, label, bare }: { value: string; onChange: (c: string) => void; disabled?: boolean; label?: string; bare?: boolean }) {
  // bare: only the colour square (click it to choose), without the "Color..." button next to it
  return (
    <label className="row" style={{ gap: 4 }}>
      {!bare && <span className="btn" style={{ padding: '2px 8px', opacity: disabled ? 0.5 : 1 }}>{label || t('Color...')}</span>}
      {/* a button with the chosen color on it; the real color input lies invisibly over it, so a click opens the picker */}
      <span className="btn" style={{ position: 'relative', width: 32, height: 24, padding: 0, justifyContent: 'center', opacity: disabled ? 0.45 : 1 }}>
        <span style={{ width: 16, height: 12, borderRadius: 2, background: value || '#ffffff', border: '1px solid var(--border-strong)', boxSizing: 'border-box' }} />
        <input type="color" value={value || '#ffffff'} disabled={disabled} onChange={(e) => onChange(e.target.value)} aria-label={label || t('Color...')}
          style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', opacity: 0, cursor: 'default', padding: 0, border: 0 }} />
      </span>
    </label>
  );
}

export const alignVOptions = () => [t('Top'), t('Middle'), t('Bottom')];
export const alignHOptions = () => [t('Left'), t('Center'), t('Right')];

// horizontal: lines of text placed left / centre / right. vertical: the line heights of two lines at the top / middle / bottom
// of the button, the second (shorter) one following the horizontal alignment
const ALIGN_ICONS = {
  h: ['M4 6h16M4 10h10M4 14h14M4 18h8', 'M4 6h16M7 10h10M5 14h14M8 18h8', 'M4 6h16M10 10h10M6 14h14M12 18h8'],
  v: [4, 9, 15],
};

// "Slots": rows stacked like the screen, the solid row is where the thing goes and the faded ones are the rest
const slot = (y: number, h: number, on: boolean) => on
  ? <rect x="5" y={y} width="14" height={h} rx="1" fill="currentColor" stroke="none" />
  : <rect x="5" y={y} width="14" height={h} rx="1" opacity="0.45" />;

/** Icons of the position choices (where a block of text sits on the screen, where a line sits next to the text). */
export const POSITION_ICONS = {
  screenTop: <>{slot(3, 5, true)}{slot(10, 4, false)}{slot(16, 5, false)}</>,
  screenCenter: <>{slot(3, 5, false)}{slot(10, 4, true)}{slot(16, 5, false)}</>,
  screenBottom: <>{slot(3, 5, false)}{slot(10, 4, false)}{slot(16, 5, true)}</>,
  above: <>{slot(3, 4, true)}{slot(10, 11, false)}</>,
  below: <>{slot(3, 11, false)}{slot(17, 4, true)}</>,
  belowText: <>{slot(3, 9, false)}{slot(14, 3, true)}</>,
  bottomOfScreen: <>{slot(3, 9, false)}{slot(18, 3, true)}</>,
};

/** A joined group of icon buttons that picks one of a few values; the buttons keep the order of items. */
export function IconChoice({ items, value, onChange, disabled }: { items: { value: number; label: string; icon: React.ReactNode }[]; value: number; onChange: (v: number) => void; disabled?: boolean }) {
  return (
    <span className="btn-group" role="group">
      {items.map((it) => (
        <button key={it.value} type="button" className={value === it.value ? 'on' : ''} title={it.label} aria-label={it.label} aria-pressed={value === it.value} disabled={disabled} onClick={() => onChange(it.value)}
          style={{ padding: '3px 7px' }}>
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true">{it.icon}</svg>
        </button>
      ))}
    </span>
  );
}

/** Alignment as a joined group of icon buttons: horizontal (left, centre, right) or vertical (top, middle, bottom). */
const vIcon = (y: number, h: number) => `M4 ${y}h16M${[4, 7, 10][h] ?? 4} ${y + 4}h10`;

/** For the vertical group, `h` is the horizontal alignment: the shorter second line of every icon sits left, centre or right accordingly. */
export function AlignGroup({ axis, value, onChange, disabled, h = 0 }: { axis: 'h' | 'v'; value: number; onChange: (v: number) => void; disabled?: boolean; h?: number }) {
  const names = axis === 'h' ? alignHOptions() : alignVOptions();
  return (
    <span className="btn-group" role="group">
      {names.map((name, i) => (
        <button key={i} type="button" className={value === i ? 'on' : ''} title={name} aria-label={name} aria-pressed={value === i} disabled={disabled} onClick={() => onChange(i)}
          style={{ padding: '3px 7px' }}>
          <svg viewBox="0 0 24 24" width="16" height="16" fill="none" stroke="currentColor" strokeWidth="2" strokeLinecap="round" strokeLinejoin="round" aria-hidden="true"><path d={axis === 'h' ? ALIGN_ICONS.h[i] : vIcon(ALIGN_ICONS.v[i], h)} /></svg>
        </button>
      ))}
    </span>
  );
}

export function Select({ value, options, onChange, disabled, style }: { value: number; options: string[]; onChange: (v: number) => void; disabled?: boolean; style?: React.CSSProperties }) {
  return (
    <select value={value} disabled={disabled} onChange={(e) => onChange(Number(e.target.value))} style={style}>
      {options.map((o, i) => (
        <option key={i} value={i}>{o}</option>
      ))}
    </select>
  );
}

/** Textarea with a highlighted backdrop (stanza/announcement titles). */
export function HighlightTextarea({ value, onChange, titleWords, rows, style, autoFocus }: { value: string; onChange: (v: string) => void; titleWords: string[]; rows?: number; style?: React.CSSProperties; autoFocus?: boolean }) {
  const [scroll, setScroll] = useState(0);
  const lines = value.split('\n');
  const isTitle = (l: string) => titleWords.some((w) => l.startsWith(w));
  const common: React.CSSProperties = {
    font: '13px/1.35 Menlo, Consolas, "Courier New", monospace',
    padding: '4px 6px',
    whiteSpace: 'pre-wrap',
    overflowWrap: 'break-word',
    margin: 0,
    border: '1px solid transparent',
  };
  return (
    <div style={{ position: 'relative', background: 'var(--base)', border: '1px solid var(--border)', borderRadius: 3, overflow: 'hidden', ...style }}>
      <div aria-hidden style={{ ...common, position: 'absolute', inset: 0, color: 'transparent', overflow: 'hidden', pointerEvents: 'none' }}>
        <div style={{ transform: `translateY(-${scroll}px)` }}>
          {lines.map((l, i) => (
            <div key={i} style={{ minHeight: '1.35em' }}>
              <span style={isTitle(l) ? { background: 'var(--stanza-title)', color: 'transparent' } : undefined}>{l || ' '}</span>
            </div>
          ))}
        </div>
      </div>
      <textarea
        value={value}
        rows={rows}
        autoFocus={autoFocus}
        spellCheck={false}
        onChange={(e) => onChange(e.target.value)}
        onScroll={(e) => setScroll(e.currentTarget.scrollTop)}
        style={{ ...common, position: 'relative', width: '100%', height: '100%', background: 'transparent', color: 'var(--text)', display: 'block', resize: 'none', outline: 'none' }}
      />
    </div>
  );
}
