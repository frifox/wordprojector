import React, { useEffect, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { t } from '../i18n';
import type { ChapterVerse, RenderSlide, Settings, Theme } from '../lib/types';
import { api } from '../lib/api';
import ScaledSlide from '../projector/ScaledSlide';
import BibleVersionButtons from '../components/BibleVersionButtons';

const SAMPLE_VERSE = 'In the beginning God created the heaven and the earth.';
// Genesis 1:1-5, numbered the way several selected verses are shown
const SAMPLE_VERSES = '(1) In the beginning God created the heaven and the earth. (2) And the earth was without form, and void; and darkness was upon the face of the deep. And the Spirit of God moved upon the face of the waters. (3) And God said, Let there be light: and there was light. (4) And God saw the light, that it was good: and God divided the light from the darkness. (5) And God called the light Day, and the darkness he called Night. And the evening and the morning were the first day.';
const SAMPLE_STANZA = 'Amazing grace, how sweet the sound\nThat saved a wretch like me\nI once was lost but now am found\nWas blind but now I see';

function background(th: Theme): RenderSlide['background'] {
  return { type: 'color', color: th.backgroundColor || '#000000' };
}

/** Projecting the sample on the real displays (the theme editor's Preview button). */
export interface PresentControl {
  on: boolean;
  toggle: () => void;
  send: (slide: RenderSlide) => void;
}

/** The Preview button, aligned to the right edge above the preview. */
function PreviewButton({ present, children }: { present?: PresentControl; children?: React.ReactNode }) {
  return (
    <div style={{ width: 'min(100%, 420px)', display: 'flex', alignItems: 'center', gap: 8 }}>
      {children}
      <span style={{ flex: 1 }} />
      {present && (
        <button className={present.on ? 'primary' : ''} aria-pressed={present.on} onClick={present.toggle}
          title={present.on ? t('Stop showing the preview on the displays') : t('Show the preview on the displays')}>{t('Preview')}</button>
      )}
    </div>
  );
}

/** While the Preview button is on, every change of the sample goes to the displays (with the transition being edited). */
function usePresenting(present: PresentControl | undefined, slide: RenderSlide | null, settings: Settings) {
  const key = JSON.stringify(slide) + JSON.stringify(settings.app.transition ?? null);
  useEffect(() => {
    if (!present?.on || !slide) return;
    const tr = settings.app.transition ?? { cut: false, fadeMillis: 500 };
    present.send({ ...slide, fade: !tr.cut, fadeMillis: tr.fadeMillis });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [present?.on, key]);
}

/** Builds the sample slide from the settings being edited (not yet saved), the way the engine would render it. */
export function sampleSlide(kind: 'bible' | 'song', th: Theme, s: Settings, multi = false, chapters: Record<string, ChapterVerse[]> = {}): RenderSlide | null {
  if (kind === 'bible') {
    const v = th.bible;
    const bv = s.bible[0];
    const ids = [bv.primary, bv.secondary, bv.trinary].filter((x) => x !== 'none');
    // the passage in each version that is lit (Genesis 1 as it is loaded; the English sample until then)
    const text = (id: string) => {
      const vs = chapters[id];
      if (!vs?.length) return multi ? SAMPLE_VERSES : SAMPLE_VERSE;
      return multi ? vs.slice(0, 5).map((v) => `(${v.num}) ${v.text}`).join(' ') : vs[0].text;
    };
    return {
      kind: 'bible', fade: false, background: background(th),
      bible: {
        parts: (ids.length ? ids : ['']).map((id) => ({ text: text(id), caption: multi ? 'Genesis 1:1-5' : 'Genesis 1:1' })),
        text: { font: v.textFont, color: v.textColor, shadow: v.useShadow, blur: v.useBlurShadow, alignV: v.alignV, alignH: v.alignH, scaleH: v.scaleH },
        captionFont: v.captionFont, captionColor: v.captionColor, captionAlign: v.captionAlign, captionPosition: v.captionPosition, captionScaleH: v.captionScaleH,
        screenUse: v.screenUse, screenPosition: v.screenPosition, addBk: v.addBkColorToText, recBk: v.textRecBkColor, bkPadding: v.bkPadding, bkOpacity: v.bkOpacity,
      },
    };
  }
  const v = th.song;
  return {
    kind: 'song', fade: false, background: background(th),
    song: {
      text: SAMPLE_STANZA, stanzaTitle: 'Verse 1', number: 123, tune: 'C', isLast: true,
      style: { font: v.textFont, color: v.textColor, shadow: v.useShadow, blur: v.useBlurShadow, alignV: v.alignV, alignH: v.alignH, scaleH: v.scaleH },
      showTitle: v.showStanzaTitle, showKey: v.showKey, showNumber: v.showNumber, infoFont: v.infoFont, infoColor: v.infoColor, infoAlign: v.infoAlign, infoScaleH: v.infoScaleH,
      showEnding: v.showEnding, endingFont: v.endingFont, endingColor: v.endingColor, endingType: v.endingType, endingPosition: v.endingPosition, endingScaleH: v.endingScaleH,
      screenUse: v.screenUse, screenPosition: v.screenPosition, addBk: v.addBkColorToText, recBk: v.textRecBkColor, bkPadding: v.bkPadding, bkOpacity: v.bkOpacity,
      wordsBy: 'John Newton', musicBy: '',
    },
  };
}

/** A live preview under a settings page: it follows every change made on the page, before it is applied. */
export default function SlidePreview({ kind, theme, settings, present }: { kind: 'bible' | 'song'; theme: Theme; settings: Settings; present?: PresentControl }) {
  const { live, settings: saved } = useStore();
  const ref = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(300);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    setW(el.clientWidth);
    return () => ro.disconnect();
  }, []);
  const d = live.displays[0];
  const aspect = d?.width && d?.height ? Math.max(1, Math.min(4, d.width / d.height)) : 16 / 9;
  const [multi, setMulti] = useState(false); // Bible: one verse or several
  // the versions shown are the ones lit on the buttons above, which are saved as soon as they are clicked
  // Genesis 1 in every version that is lit
  const [chapters, setChapters] = useState<Record<string, ChapterVerse[]>>({});
  const bv = saved.bible[0];
  const lit = [bv.primary, bv.secondary, bv.trinary].filter((x) => x !== 'none').join(',');
  useEffect(() => {
    if (kind !== 'bible') return;
    let live = true;
    lit.split(',').filter((id) => id && !chapters[id]).forEach((id) =>
      api.chapter(Number(id), 1, 1).then((vs) => { if (live) setChapters((c) => ({ ...c, [id]: vs })); }).catch(() => {}));
    return () => { live = false; };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [kind, lit]);
  const slide = sampleSlide(kind, theme, kind === 'bible' ? { ...settings, bible: saved.bible } : settings, multi, chapters);
  usePresenting(present, slide, settings);
  return (
    <div className="col" style={{ gap: 4, flex: 'none', alignItems: 'center' }}>
      <PreviewButton present={present} />
      <div ref={ref} style={{ width: 'min(100%, 420px)', aspectRatio: String(aspect), position: 'relative', overflow: 'hidden', background: '#000', border: '1px solid var(--border)', boxSizing: 'border-box' }}>
        {slide && <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}><ScaledSlide slide={slide} width={w} height={w / aspect} /></div>}
      </div>
      {kind === 'bible' && (
        <div style={{ marginTop: 4, display: 'flex', alignItems: 'center', gap: 12, flexWrap: 'wrap', justifyContent: 'center' }}>
          <span className="btn-group" role="group">
            <button type="button" className={!multi ? 'on' : ''} aria-pressed={!multi} onClick={() => setMulti(false)}>Genesis 1:1</button>
            <button type="button" className={multi ? 'on' : ''} aria-pressed={multi} onClick={() => setMulti(true)}>Genesis 1:1-5</button>
          </span>
          <BibleVersionButtons large />
        </div>
      )}
    </div>
  );
}

const STANZAS = [
  'Amazing grace, how sweet the sound\nThat saved a wretch like me\nI once was lost but now am found\nWas blind but now I see',
  "'Twas grace that taught my heart to fear\nAnd grace my fears relieved\nHow precious did that grace appear\nThe hour I first believed",
  "Through many dangers, toils and snares\nI have already come\n'Tis grace hath brought me safe thus far\nAnd grace will lead me home",
  "When we've been there ten thousand years\nBright shining as the sun\nWe've no less days to sing God's praise\nThan when we'd first begun",
];

/**
 * The Amazing Grace stanzas to step through with the arrows, changing the way the user chose in Settings (cut, or a
 * fade of the chosen time). Slides stack like on the display, so a fade in progress is never cut short.
 */
export function TransitionPreview({ theme, settings, present }: { theme: Theme; settings: Settings; present?: PresentControl }) {
  const { live } = useStore();
  const ref = useRef<HTMLDivElement>(null);
  const [w, setW] = useState(300);
  const [idx, setIdx] = useState(0);
  // one layer per slide, stacked: a new one fades in over the others and the ones under it go once it is opaque
  const [layers, setLayers] = useState<{ id: number; stanza: number }[]>([{ id: 0, stanza: 0 }]);
  const nextId = useRef(1);
  const timers = useRef<number[]>([]);
  useEffect(() => () => timers.current.forEach((id) => window.clearTimeout(id)), []);
  useEffect(() => {
    const el = ref.current;
    if (!el) return;
    const ro = new ResizeObserver(() => setW(el.clientWidth));
    ro.observe(el);
    setW(el.clientWidth);
    return () => ro.disconnect();
  }, []);
  const go = (to: number) => {
    const n = ((to % STANZAS.length) + STANZAS.length) % STANZAS.length;
    setIdx(n);
    const layer = { id: nextId.current++, stanza: n };
    setLayers((l) => [...l, layer]);
    const ms = settings.app.transition?.cut ? 0 : (settings.app.transition?.fadeMillis ?? 500) + 30;
    timers.current.push(window.setTimeout(() => setLayers((l) => l.filter((x) => x.id >= layer.id)), ms));
  };
  // moves on by itself every 3 s, round and round; using the arrows starts the 3 s again
  useEffect(() => {
    const timer = window.setTimeout(() => go(idx + 1), 3000);
    return () => window.clearTimeout(timer);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [idx]);
  const d = live.displays[0];
  const aspect = d?.width && d?.height ? Math.max(1, Math.min(4, d.width / d.height)) : 16 / 9;
  const tr = settings.app.transition ?? { cut: false, fadeMillis: 500 };
  const slideAt = (i: number): RenderSlide | null => {
    const sl = sampleSlide('song', theme, settings);
    if (sl?.song) { sl.song.text = STANZAS[i]; sl.song.stanzaTitle = `Verse ${i + 1}`; sl.song.isLast = i === STANZAS.length - 1; }
    return sl;
  };
  usePresenting(present, slideAt(idx), settings);
  return (
    <div className="col" style={{ gap: 6, flex: 'none', alignItems: 'center' }}>
      <PreviewButton present={present} />
      <div ref={ref} style={{ width: 'min(100%, 420px)', aspectRatio: String(aspect), position: 'relative', overflow: 'hidden', background: '#000', border: '1px solid var(--border)', boxSizing: 'border-box' }}>
        {layers.map((l) => {
          const slide = slideAt(l.stanza);
          return (
            <div key={l.id} style={{ position: 'absolute', inset: 0, pointerEvents: 'none', animation: l.id > 0 && !tr.cut ? `wp-fade-in ${tr.fadeMillis / 1000}s ease both` : 'none', willChange: 'opacity' }}>
              {slide && <ScaledSlide slide={slide} width={w} height={w / aspect} />}
            </div>
          );
        })}
      </div>
      <div className="row" style={{ gap: 12, alignItems: 'center' }}>
        <button className="tbtn-s" title={t('Previous stanza')} aria-label={t('Previous stanza')} onClick={() => go(idx - 1)}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M15 6l-6 6l6 6" /></svg>
        </button>
        <span className="muted">{t('Verse {n} of {total}', { n: idx + 1, total: STANZAS.length })}</span>
        <button className="tbtn-s" title={t('Next stanza')} aria-label={t('Next stanza')} onClick={() => go(idx + 1)}>
          <svg viewBox="0 0 24 24" aria-hidden="true"><path d="M9 6l6 6l-6 6" /></svg>
        </button>
      </div>
    </div>
  );
}
