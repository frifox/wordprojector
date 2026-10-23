import React, { useLayoutEffect, useMemo, useState } from 'react';
import type { AnnounceSlide, BibleSlide, RenderSlide, SongSlide } from '../lib/types';
import { alignCss, endingStrings, fitText, fitWrapped, fontCss, fontPx, fontProps, justifyCss, lineBox, measureText, shadowCss } from './fit';

/**
 * Renders one slide at a given pixel size. Used full screen by the projector
 * window and scaled down by the operator's live preview.
 */
export default function SlideView({ slide, width, height }: { slide: RenderSlide; width: number; height: number }) {
  const bg = slide.background;
  const style: React.CSSProperties = {
    position: 'absolute',
    inset: 0,
    width,
    height,
    overflow: 'hidden',
    background: bg.type === 'color' ? bg.color || '#000' : '#000',
  };
  return (
    <div style={style}>
      {bg.type === 'image' && bg.url && (
        <img src={bg.url} alt="" style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: bg.fit || 'fill' }} draggable={false} />
      )}
      {slide.kind === 'test' && slide.test && (
        <div style={{ position: 'absolute', inset: 0, display: 'flex', flexDirection: 'column', alignItems: 'center', justifyContent: 'center', color: '#fff', gap: height * 0.03,
          background: 'repeating-linear-gradient(45deg, #101010, #101010 40px, #181818 40px, #181818 80px)', border: `${Math.max(3, height * 0.012)}px solid #4a8ae0`, boxSizing: 'border-box', fontFamily: 'sans-serif' }}>
          <div style={{ fontSize: height * 0.3, fontWeight: 700, lineHeight: 1 }}>{slide.test.disp}</div>
          <div style={{ fontSize: height * 0.07 }}>Word Projector</div>
          <div style={{ fontSize: height * 0.045, opacity: 0.7 }}>Display {slide.test.disp} · screen {slide.test.screen + 1}{slide.test.width ? ` · ${slide.test.width} × ${slide.test.height}` : ''}</div>
          <CountdownDisc size={height * 0.1} millis={slide.test.millis || 3000} />
        </div>
      )}
      {slide.kind === 'bible' && slide.bible && <BibleView s={slide.bible} width={width} height={height} />}
      {slide.kind === 'song' && slide.song && <SongView s={slide.song} width={width} height={height} />}
      {slide.kind === 'announce' && slide.announce && <AnnounceView s={slide.announce} width={width} height={height} />}
      {slide.kind === 'picture' && slide.picture && <PictureView url={slide.picture.url} w={slide.picture.width} h={slide.picture.height} expandSmall={slide.picture.expandSmall} fitType={slide.picture.fitType} width={width} height={height} />}
    </div>
  );
}

const MARGIN_X = 30;
const MARGIN_Y = 20;

function useBox(width: number, height: number, screenUse: number, screenPosition: number) {
  return useMemo(() => {
    const w = width - MARGIN_X * 2;
    const h = height - MARGIN_Y * 2;
    const use = screenUse > 0 && screenUse <= 100 ? screenUse : 100;
    const maxh = Math.round((h * use) / 100);
    // 0 = top, 1 = bottom, 2 = centre of the screen
    const top = screenPosition === 1 ? MARGIN_Y + h - maxh : screenPosition === 2 ? MARGIN_Y + Math.round((h - maxh) / 2) : MARGIN_Y;
    return { left: MARGIN_X, top, w, h: maxh };
  }, [width, height, screenUse, screenPosition]);
}

/**
 * Horizontal scale: the characters are that percent as wide as normal. The text is laid out in a box 100 / scale times
 * as wide and squeezed (stretched) back onto the screen area, so it wraps and fits as if the characters really were
 * that width.
 */
function textArea(box: { left: number; w: number }, scale = 100) {
  const k = Math.max(0.2, Math.min(3, (scale || 100) / 100));
  const w = box.w / k;
  return { left: box.left + (box.w - w) / 2, w, k, css: k === 1 ? {} : { transform: `scaleX(${k})`, transformOrigin: 'center' } as React.CSSProperties };
}

function BibleView({ s, width, height }: { s: BibleSlide; width: number; height: number }) {
  const box = useBox(width, height, s.screenUse, s.screenPosition);
  const n = Math.max(1, s.parts.length);
  const partH = Math.floor(box.h / n);
  const tx = textArea(box, s.text.scaleH);
  const ca = textArea(box, s.captionScaleH);
  const [fit, setFit] = useState<{ textPx: number; capPx: number; parts: { th: number; ch: number }[] } | null>(null);
  const key = JSON.stringify([s.parts, s.text.font, s.captionFont, box, partH, tx.w, ca.w]);
  useLayoutEffect(() => {
    // Shrink text (and caption when larger than text) until every part fits.
    let textPx = fontPx(s.text.font);
    let capPx = fontPx(s.captionFont);
    const measureAll = () =>
      s.parts.map((p) => {
        const c = measureText(p.caption, fontCss(s.captionFont, capPx), ca.w, true);
        const t = measureText(p.text, fontCss(s.text.font, textPx), tx.w, true);
        return { th: t.height, ch: c.height };
      });
    let parts = measureAll();
    let guard = 0;
    while (parts.some((p) => p.th + p.ch > partH) && textPx > 8 && guard < 400) {
      textPx -= Math.max(1, Math.round(textPx * 0.04));
      if (capPx > textPx) capPx -= Math.max(1, Math.round(capPx * 0.04));
      parts = measureAll();
      guard++;
    }
    setFit({ textPx, capPx, parts });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  if (!fit) return null;
  return (
    <>
      {s.parts.map((p, i) => {
        const top = box.top + i * partH;
        const f = fit.parts[i] || { th: 0, ch: 0 };
        const total = f.th + f.ch;
        // vertical alignment of the (text+caption) block inside the part
        let y = top;
        if (s.text.alignV === 1) y = top + Math.max(0, (partH - total) / 2);
        else if (s.text.alignV === 2) y = top + Math.max(0, partH - total);
        const captionFirst = s.captionPosition === 0;
        const textStyle: React.CSSProperties = {
          position: 'absolute', left: tx.left, width: tx.w, ...tx.css, top: captionFirst ? y + f.ch : y,
          ...fontProps(s.text.font, fit.textPx), lineHeight: 1.2, color: s.text.color, textAlign: alignCss(s.text.alignH),
          whiteSpace: 'pre-wrap', overflowWrap: 'break-word', textShadow: shadowCss(s.text.shadow, s.text.blur, fit.textPx),
        };
        const capStyle: React.CSSProperties = {
          position: 'absolute', left: ca.left, width: ca.w, ...ca.css, top: captionFirst ? y : y + f.th,
          ...fontProps(s.captionFont, fit.capPx), lineHeight: 1.2, color: s.captionColor, textAlign: alignCss(s.captionAlign),
          whiteSpace: 'pre-wrap', textShadow: shadowCss(s.text.shadow, s.text.blur, fit.capPx),
        };
        return (
          <React.Fragment key={i}>
            {s.addBk && (
              // the color goes behind the text and its caption, with the padding above and below them
              <div style={{ position: 'absolute', left: 0, width, top: y - (s.bkPadding ?? 10), height: total + 2 * (s.bkPadding ?? 10), background: s.recBk, opacity: (s.bkOpacity ?? 100) / 100 }} />
            )}
            <div style={textStyle}>{p.text}</div>
            <div style={capStyle}>{p.caption}</div>
          </React.Fragment>
        );
      })}
    </>
  );
}

function SongView({ s, width, height }: { s: SongSlide; width: number; height: number }) {
  const box = useBox(width, height, s.screenUse, s.screenPosition);
  const tx = textArea(box, s.style.scaleH);
  const ia = textArea(box, s.infoScaleH);
  const ea = textArea(box, s.endingScaleH);
  const showInfo = s.showTitle || s.showKey || s.showNumber;
  const caption = s.showTitle ? s.stanzaTitle : showInfo ? ' ' : '';
  let numStr = s.showNumber ? String(s.number) : ' ';
  if (s.showKey) numStr = `${s.tune}  ${numStr}`;
  if (!showInfo) numStr = '';
  let ending = '';
  if (s.showEnding) {
    ending = ' ';
    if (s.isLast) {
      if (s.endingType === 7) {
        const w = s.wordsBy ? `Words by: ${s.wordsBy}` : '';
        const m = s.musicBy ? `Music by: ${s.musicBy}` : '';
        ending = [w, m].filter(Boolean).join(', ') || endingStrings[0];
      } else ending = endingStrings[s.endingType] || endingStrings[0];
    }
  }
  const [fit, setFit] = useState<{ px: number; wrap: boolean; th: number; caph: number; endh: number; endPx: number } | null>(null);
  const key = JSON.stringify([s.text, s.stanzas, s.style, caption, numStr, ending, s.infoFont, s.endingFont, box, tx.w, ea.w]);
  useLayoutEffect(() => {
    const infoPx = fontPx(s.infoFont);
    const caph = showInfo ? lineBox(infoPx) : 0;
    let endPx = fontPx(s.endingFont);
    let endh = 0;
    if (ending) {
      let m = measureText(ending, fontCss(s.endingFont, endPx), ea.w, false);
      while (m.width > ea.w && endPx > 8) {
        endPx -= 2;
        m = measureText(ending, fontCss(s.endingFont, endPx), ea.w, false);
      }
      endh = lineBox(endPx);
    }
    // Fit every stanza of the song and use the smallest size (and wrapping if any stanza needs it) for all of them,
    // so the text keeps one size and one line height from stanza to stanza.
    const texts = Array.from(new Set([s.text, ...(s.stanzas || [])]));
    const fits = texts.map((txt) => fitText(txt, s.style.font, tx.w, box.h, caph + endh));
    const px = Math.min(...fits.map((f) => f.px));
    const wrap = fits.some((f) => f.wrap);
    const cur = measureText(s.text, fontCss(s.style.font, px), tx.w, wrap);
    setFit({ px, wrap, th: cur.height, caph, endh, endPx });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  if (!fit) return null;
  const infoPx = fontPx(s.infoFont);
  const mainH = box.h - fit.caph - fit.endh;
  const infoTop = s.infoAlign === 0;
  const endingBottom = s.endingPosition === 1;
  // main text block placement
  const mainTop = infoTop ? box.top + fit.caph : box.top;
  const mainStyle: React.CSSProperties = {
    position: 'absolute', left: tx.left, width: tx.w, ...tx.css, top: mainTop, height: mainH,
    display: 'flex', alignItems: justifyCss(s.style.alignV), justifyContent: alignCss(s.style.alignH) === 'left' ? 'flex-start' : alignCss(s.style.alignH) === 'right' ? 'flex-end' : 'center',
  };
  const textStyle: React.CSSProperties = {
    ...fontProps(s.style.font, fit.px), lineHeight: 1.2, color: s.style.color, textAlign: alignCss(s.style.alignH),
    whiteSpace: fit.wrap ? 'pre-wrap' : 'pre', overflowWrap: fit.wrap ? 'break-word' : 'normal', maxWidth: '100%',
    textShadow: shadowCss(s.style.shadow, s.style.blur, fit.px),
  };
  const infoStyle = (align: 'left' | 'right'): React.CSSProperties => ({
    position: 'absolute', left: ia.left, width: ia.w, ...ia.css, top: infoTop ? box.top : box.top + box.h - fit.caph,
    ...fontProps(s.infoFont, infoPx), lineHeight: `${lineBox(infoPx)}px`, height: lineBox(infoPx), color: s.infoColor, textAlign: align, whiteSpace: 'pre',
    textShadow: shadowCss(s.style.shadow, s.style.blur, infoPx),
  });
  let endTop: number;
  if (endingBottom) endTop = box.top + box.h - fit.endh - (infoTop ? 0 : fit.caph);
  else {
    // below song text: compute bottom of the text block according to vertical alignment
    let textBottom = mainTop + fit.th;
    if (s.style.alignV === 1) textBottom = mainTop + (mainH - fit.th) / 2 + fit.th;
    else if (s.style.alignV === 2) textBottom = mainTop + mainH;
    endTop = textBottom;
  }
  // the color goes behind the text and the ending (when it is shown), not the caption, with the padding around them
  let textTop = mainTop;
  if (s.style.alignV === 1) textTop = mainTop + (mainH - fit.th) / 2;
  else if (s.style.alignV === 2) textTop = mainTop + mainH - fit.th;
  let bkTop = textTop;
  let bkBottom = textTop + fit.th;
  if (ending.trim()) { // only a visible ending (the last stanza's)
    bkTop = Math.min(bkTop, endTop);
    bkBottom = Math.max(bkBottom, endTop + fit.endh);
  }
  const bkPad = s.bkPadding ?? 10;
  const endStyle: React.CSSProperties = {
    position: 'absolute', left: ea.left, width: ea.w, ...ea.css, top: endTop, ...fontProps(s.endingFont, fit.endPx), lineHeight: `${lineBox(fit.endPx)}px`, height: lineBox(fit.endPx),
    color: s.endingColor, textAlign: 'center', whiteSpace: 'pre', textShadow: shadowCss(s.style.shadow, s.style.blur, fit.endPx),
  };
  return (
    <>
      {s.addBk && <div style={{ position: 'absolute', left: 0, width, top: bkTop - bkPad, height: bkBottom - bkTop + 2 * bkPad, background: s.recBk, opacity: (s.bkOpacity ?? 100) / 100 }} />}
      {showInfo && <div style={infoStyle('left')}>{caption}</div>}
      {showInfo && <div style={infoStyle('right')}>{numStr}</div>}
      <div style={mainStyle}>
        <div style={textStyle}>
          {fit.wrap ? s.text : s.text.split('\n').map((line, i) => (
            <div key={i} style={{ height: lineBox(fit.px), lineHeight: `${lineBox(fit.px)}px` }}>{line || ' '}</div>
          ))}
        </div>
      </div>
      {ending && <div style={endStyle}>{ending}</div>}
    </>
  );
}

function AnnounceView({ s, width, height }: { s: AnnounceSlide; width: number; height: number }) {
  const box = useBox(width, height, 100, 0);
  const [fit, setFit] = useState<{ px: number; wrap: boolean; h: number } | null>(null);
  const key = JSON.stringify([s.text, s.style, box]);
  useLayoutEffect(() => {
    const r = fitWrapped(s.text, s.style.font, box.w, box.h);
    setFit({ px: r.px, wrap: true, h: r.height });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [key]);
  if (!fit) return null;
  const wrap: React.CSSProperties = {
    position: 'absolute', left: box.left, top: box.top, width: box.w, height: box.h, display: 'flex',
    alignItems: justifyCss(s.style.alignV), justifyContent: alignCss(s.style.alignH) === 'left' ? 'flex-start' : alignCss(s.style.alignH) === 'right' ? 'flex-end' : 'center',
  };
  const text: React.CSSProperties = {
    ...fontProps(s.style.font, fit.px), lineHeight: 1.2, color: s.style.color, textAlign: alignCss(s.style.alignH), whiteSpace: 'pre-wrap',
    overflowWrap: 'break-word', maxWidth: '100%', textShadow: shadowCss(s.style.shadow, s.style.blur, fit.px),
  };
  return (
    <div style={wrap}>
      <div style={text}>{s.text}</div>
    </div>
  );
}

function PictureView({ url, w, h, expandSmall, fitType, width, height }: { url: string; w: number; h: number; expandSmall: boolean; fitType: number; width: number; height: number }) {
  const small = w > 0 && h > 0 && w < width && h < height;
  const expand = small ? expandSmall : true;
  let objectFit: React.CSSProperties['objectFit'] = 'contain';
  let size: React.CSSProperties = { width: '100%', height: '100%' };
  if (!expand) {
    size = { width: w, height: h, left: (width - w) / 2, top: (height - h) / 2 };
    objectFit = 'fill';
  } else if (fitType === 0) objectFit = 'contain';
  else if (fitType === 1) objectFit = 'cover';
  else objectFit = 'fill';
  // fitType: 0 = keep aspect, 1 = keep aspect by expanding, 2 = stretch (matches setBackPixmap modes 1,2,0)
  return <img src={url} alt="" draggable={false} style={{ position: 'absolute', objectFit, ...size }} />;
}

export function slideKey(slide: RenderSlide): string {
  return JSON.stringify(slide);
}

/** A filled disc that empties clockwise from 12 o'clock over `millis`. */
export function CountdownDisc({ size, millis }: { size: number; millis: number }) {
  return (
    <svg viewBox="0 0 16 16" width={size} height={size} aria-hidden="true" style={{ display: 'block' }}>
      <circle cx="8" cy="8" r="4" fill="none" stroke="currentColor" strokeWidth="8" pathLength={100} strokeDasharray="100" transform="rotate(-90 8 8)"
        style={{ animation: `wp-countdown ${millis}ms linear forwards` }} />
    </svg>
  );
}
