// Text fitting helpers that reproduce the original ImageGenerator behaviour:
// fonts are specified in points, rendered at 3x on the display, and shrunk
// until the text fits. If shrinking goes below 4/5 of the original size the
// text is laid out again with word wrapping enabled.

import type { Font } from '../lib/types';

/** Pixel size for a font point size on the projector (pt * 3 at 96 dpi). */
export function fontPx(f: Font, scale = 1): number {
  const pt = f.pointSize > 0 ? f.pointSize : 16;
  return pt * 4 * scale;
}

/** The same font as separate style properties. Mixing the `font` shorthand with `line-height` in one React style
 *  object lets a re-render reset the line height, which made lines jump between slides. */
export function fontProps(f: Font, px: number): { fontStyle: string; fontWeight: number; fontSize: number; fontFamily: string } {
  return { fontStyle: f.italic ? 'italic' : 'normal', fontWeight: f.bold ? 700 : 400, fontSize: px, fontFamily: `${f.family ? `"${f.family}", ` : ''}Arial, sans-serif` };
}

export function fontCss(f: Font, px: number): string {
  const style = f.italic ? 'italic ' : '';
  const weight = f.bold ? '700 ' : '400 ';
  const fam = f.family ? `"${f.family}", ` : '';
  return `${style}${weight}${px}px ${fam}Arial, sans-serif`;
}

let measurer: HTMLDivElement | null = null;

function getMeasurer(): HTMLDivElement {
  if (!measurer) {
    measurer = document.createElement('div');
    measurer.style.position = 'absolute';
    measurer.style.visibility = 'hidden';
    measurer.style.left = '-100000px';
    measurer.style.top = '0';
    measurer.style.pointerEvents = 'none';
    document.body.appendChild(measurer);
  }
  return measurer;
}

export interface Measure {
  width: number;
  height: number;
}

/** Measures a block of text at the given font, optionally constrained to a width with wrapping. */
export function measureText(text: string, font: string, maxWidth: number, wrap: boolean, lineHeight = 1.2): Measure {
  const m = getMeasurer();
  m.style.font = font;
  m.style.lineHeight = String(lineHeight);
  m.style.whiteSpace = wrap ? 'pre-wrap' : 'pre';
  m.style.width = wrap ? `${maxWidth}px` : 'auto';
  m.style.maxWidth = wrap ? `${maxWidth}px` : 'none';
  m.style.wordBreak = 'normal';
  m.style.overflowWrap = wrap ? 'break-word' : 'normal';
  m.textContent = text || ' ';
  const r = m.getBoundingClientRect();
  if (!wrap) {
    // unwrapped text is drawn in fixed-height lines (see lineBox), so its height is just the line count
    const px = parseFloat(/([\d.]+)px/.exec(font)?.[1] || '0');
    if (px > 0) return { width: Math.ceil(r.width), height: Math.max(1, text.split('\n').length) * lineBox(px, lineHeight) };
  }
  return { width: Math.ceil(r.width), height: Math.ceil(r.height) };
}

/** Height of one line of text in px. Using one fixed value everywhere keeps lines the same height from slide to slide. */
export function lineBox(px: number, lineHeight = 1.2): number {
  return Math.max(1, Math.round(px * lineHeight));
}

export interface FitResult {
  px: number;
  wrap: boolean;
  width: number;
  height: number;
}

/**
 * Shrink the font until the text fits in maxWidth x maxHeight (optionally
 * accounting for extra fixed height taken by other elements). Port of the
 * song/announcement fitting loops.
 */
export function fitText(text: string, f: Font, maxWidth: number, maxHeight: number, extraHeight = 0, minPx = 8): FitResult {
  const orig = fontPx(f);
  const tryFit = (wrap: boolean): FitResult => {
    let px = orig;
    let m = measureText(text, fontCss(f, px), maxWidth, wrap);
    while ((m.width > maxWidth || m.height + extraHeight > maxHeight) && px > minPx) {
      px -= Math.max(1, Math.round(px * 0.04));
      m = measureText(text, fontCss(f, px), maxWidth, wrap);
    }
    return { px, wrap, width: m.width, height: m.height };
  };
  let r = tryFit(false);
  if (r.px < (orig * 4) / 5) {
    r = tryFit(true);
  }
  return r;
}

/** Shrinks a wrapped block (bible text) until it fits. */
export function fitWrapped(text: string, f: Font, maxWidth: number, maxHeight: number, extraHeight = 0, minPx = 8): FitResult {
  let px = fontPx(f);
  let m = measureText(text, fontCss(f, px), maxWidth, true);
  while (m.height + extraHeight > maxHeight && px > minPx) {
    px -= Math.max(1, Math.round(px * 0.04));
    m = measureText(text, fontCss(f, px), maxWidth, true);
  }
  return { px, wrap: true, width: m.width, height: m.height };
}

export const endingStrings = ['*    *    *', '-    -    -', '°    °    °', '•    •    •', '●    ●    ●', '▪    ▪    ▪', '■    ■    ■'];

export function alignCss(alignH: number): 'left' | 'center' | 'right' {
  return alignH === 0 ? 'left' : alignH === 2 ? 'right' : 'center';
}

export function justifyCss(alignV: number): 'flex-start' | 'center' | 'flex-end' {
  return alignV === 0 ? 'flex-start' : alignV === 2 ? 'flex-end' : 'center';
}

export function shadowCss(shadow: boolean, blur: boolean, px: number): string {
  if (!shadow && !blur) return 'none';
  const off = Math.max(1, Math.round(px / 20));
  return blur ? `${off}px ${off}px ${Math.max(2, Math.round(px / 10))}px rgba(0,0,0,0.9)` : `${off}px ${off}px 0 rgba(0,0,0,0.9)`;
}
