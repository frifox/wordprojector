import StanzaText from '../components/StanzaText';
import BibleVersionButtons from '../components/BibleVersionButtons';
import { useEffect, useMemo, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';
import { Icon, useUI } from '../lib/ui';
import ScaledSlide from '../projector/ScaledSlide';
import type { RenderSlide } from '../lib/types';


// set once the hint of the Bible version buttons has played (it plays the first time the tab is opened after a start)
let bibleHintDone = false;

/** The right hand "live" area: what is projected now. */
export default function LivePanel() {
  const { live, settings } = useStore();
  const ui = useUI();
  const listRef = useRef<HTMLDivElement>(null);
  // the arrow keys step through the slides unless the focus is in a list or table of its own or in a text field (see
  // Operator): only then does the highlight of what is playing go quiet
  const [quiet, setQuiet] = useState(false);
  useEffect(() => {
    const check = () => {
      const a = document.activeElement as HTMLElement | null;
      if (a?.closest?.('.modal')) return; // a dialog is open: the highlight behind it stays as it was
      const tag = a?.tagName;
      const typing = tag === 'INPUT' || tag === 'TEXTAREA' || tag === 'SELECT' || !!a?.isContentEditable;
      const inOtherList = !!a?.closest?.('.list, .table') && !listRef.current?.contains(a);
      setQuiet(typing || inOtherList);
    };
    check();
    const later = () => { window.setTimeout(check, 0); };
    document.addEventListener('focusin', check);
    document.addEventListener('focusout', later);
    return () => {
      document.removeEventListener('focusin', check);
      document.removeEventListener('focusout', later);
    };
  }, []);
  // the first time the Bible tab is opened after the program starts, the version buttons show over the preview for 0.5 s and
  // then fade out: a hint that they are there when the pointer is over the box. 0 = not shown, 1 = shown, 2 = fading out
  const [hint, setHint] = useState(0);
  useEffect(() => {
    if (ui.tab !== 'bible' || bibleHintDone) return;
    setHint(1);
    const fade = window.setTimeout(() => { bibleHintDone = true; setHint(2); }, 500);
    const done = window.setTimeout(() => setHint(0), 1600);
    return () => { window.clearTimeout(fade); window.clearTimeout(done); setHint(0); };
  }, [ui.tab]);
  const anchor = useRef<number>(-1);
  const [multi, setMulti] = useState(false);
  const [pending, setPending] = useState<number[]>(live.selection);
  const pendingRef = useRef(pending);
  pendingRef.current = pending;
  const liveKey = live.selection.join(',') + '|' + live.items.length + '|' + live.title;
  // whenever the projected selection changes (or a new chapter is shown) the pending selection restarts from it
  useEffect(() => {
    setPending(live.selection);
    // a multi-verse selection that went live means the operator is in multi-verse mode
    if (live.type === 'bible' && live.selection.length > 1) setMulti(true);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [liveKey]);
  const isBible = live.type === 'bible';


  // Update is also available when the display was turned off/hidden, to push the selection back out
  // a staged selection that differs from the live one; independent of whether the displays are on
  const dirty = isBible && multi && pending.join(',') !== live.selection.join(',');
  // leaving multi-verse mode collapses what is shown to its first verse
  const toggleMulti = (on: boolean) => {
    setMulti(on);
    if (on || !isBible) return;
    const first = Math.min(...(live.selection.length ? live.selection : pendingRef.current));
    if (!Number.isFinite(first)) return;
    if (live.selection.length > 1) api.liveSelection([first], live.showing);
    else setPending(live.selection);
    if (live.selection.length > 1) setPending([first]);
  };
  // momentary Preview: while the button is held the preview box shows the staged selection
  const [peek, setPeek] = useState<RenderSlide | null>(null);
  const holding = useRef(false);
  // who started the hold: a press on the button (pointer) or the Space key in the list (key). Moving focus away only
  // ends a key hold: pressing the button itself takes the focus off the list, which must not cancel that press.
  const holdSource = useRef<'pointer' | 'key'>('pointer');
  const holdPreview = (on: boolean, source: 'pointer' | 'key' = 'pointer') => {
    if (on) holdSource.current = source;
    holding.current = on;
    if (!on) { setPeek(null); return; }
    api.previewSelection(pendingRef.current).then((s) => { if (holding.current) setPeek(s); }).catch(() => {});
  };
  const update = () => {
    api.liveSelection(pendingRef.current, true);
  };
  // F5 applies the pending selection (see Operator)
  const dirtyRef = useRef(dirty);
  dirtyRef.current = dirty;
  useEffect(() => {
    const h = (e: Event) => {
      const d = (e as CustomEvent).detail as { handled: boolean };
      if (dirtyRef.current) {
        update();
        d.handled = true;
      }
    };
    window.addEventListener('wp-live-update', h);
    return () => window.removeEventListener('wp-live-update', h);
  }, []);

  // a single verse sent from the Bible tab (Go Live, double-click) is a new thing to show: multi-verse mode is over
  useEffect(() => {
    const h = (e: Event) => { if (((e as CustomEvent).detail as number) <= 1) setMulti(false); };
    window.addEventListener('wp-bible-shown', h);
    return () => window.removeEventListener('wp-bible-shown', h);
  }, []);

  // after something goes live the live list takes focus so the arrow keys step through it
  useEffect(() => {
    const h = () => window.setTimeout(() => listRef.current?.focus({ preventScroll: true }), 0);
    window.addEventListener('wp-focus-live', h);
    return () => window.removeEventListener('wp-focus-live', h);
  }, []);
  const onListKey = (e: React.KeyboardEvent) => {
    // multi-verse mode: Space is the momentary Preview button while it is held
    if (e.key === ' ' && isBible && multi) {
      e.preventDefault();
      if (pending.length && !e.repeat) holdPreview(true, 'key');
      return;
    }
    // multi-verse mode: Enter applies a new selection, like Update
    if (e.key === 'Enter' && isBible && multi) {
      e.preventDefault();
      if (dirty || !live.showing) update();
      return;
    }
    // Enter on the selected row starts the presentation when it is off
    if (e.key === 'Enter' && !live.showing && live.items.length) {
      e.preventDefault();
      if (isBible && multi) update();
      else api.liveIndex(live.selection.length ? Math.min(...live.selection) : live.index, true);
      return;
    }
    if (!['ArrowDown', 'ArrowUp', 'ArrowRight', 'ArrowLeft'].includes(e.key)) return;
    e.preventDefault();
    const d = e.key === 'ArrowDown' || e.key === 'ArrowRight' ? 1 : -1;
    const n = live.items.length;
    if (isBible && multi) {
      const cur = pendingRef.current;
      // Shift extends the selection from the verse it started at (the other end follows the arrow)
      if (e.shiftKey && cur.length) {
        const from = cur.includes(anchor.current) ? anchor.current : cur[0];
        const head = from === Math.min(...cur) ? Math.max(...cur) : Math.min(...cur);
        anchor.current = from;
        setPending(range(from, Math.max(0, Math.min(n - 1, head + d))));
        return;
      }
      const base = cur.length ? (d > 0 ? Math.max(...cur) : Math.min(...cur)) : -1;
      const next = Math.max(0, Math.min(n - 1, base + d));
      anchor.current = next;
      setPending([next]);
      return;
    }
    const sel = live.selection.length ? (d > 0 ? Math.max(...live.selection) : Math.min(...live.selection)) : live.index;
    const next = Math.max(0, Math.min(n - 1, sel + d));
    if (next !== sel) api.liveIndex(next);
  };

  useEffect(() => {
    const el = listRef.current?.querySelector('.item.sel') as HTMLElement | null;
    el?.scrollIntoView({ block: 'nearest' });
  }, [live.index, live.items.length]);

  const range = (a: number, b: number) => {
    const out: number[] = [];
    for (let k = Math.min(a, b); k <= Math.max(a, b); k++) out.push(k);
    return out;
  };
  const dragSel = useRef<number[]>([]);
  // Bible: press selects, dragging or shift+click extends; the range is projected on release.
  const showingRef = useRef(live.showing);
  showingRef.current = live.showing;
  const multiRef = useRef(multi);
  multiRef.current = multi;
  const dragging = useRef(false);
  const switched = useRef(false);
  const onDown = (i: number, e: React.MouseEvent) => {
    if (e.button !== 0) return;
    if (live.type !== 'bible') {
      anchor.current = i;
      api.liveIndex(i);
      return;
    }
    const shift = e.shiftKey && anchor.current >= 0;
    if (shift) dragSel.current = range(anchor.current, i);
    else {
      anchor.current = i;
      dragSel.current = [i];
    }
    switched.current = false;
    if (multiRef.current) setPending(dragSel.current); // multi-verse mode only stages; Update / F5 projects
    else if (shift) api.liveSelection(dragSel.current, false);
    // a plain press outside multi-verse mode waits: a click projects on release,
    // a drag across verses switches to multi-verse mode without touching the display
    const wasMulti = multiRef.current;
    const up = () => {
      window.removeEventListener('mouseup', up);
      dragging.current = false;
      if (wasMulti || switched.current) return;
      // while the presentation is off a click only selects; Play, F5 or a double click starts it
      api.liveSelection(dragSel.current, showingRef.current);
    };
    dragging.current = true;
    window.addEventListener('mouseup', up);
  };
  const onEnter = (i: number) => {
    if (!dragging.current || live.type !== 'bible') return;
    const sel = range(anchor.current, i);
    if (multiRef.current) {
      dragSel.current = sel;
      setPending(sel);
    } else if (sel.length > 1 && !switched.current) {
      switched.current = true;
      setMulti(true);
      dragSel.current = sel;
      setPending(sel);
    } else if (switched.current) {
      dragSel.current = sel;
      setPending(sel);
    } else {
      dragSel.current = sel;
      api.liveSelection(sel, false);
    }
  };

  return (
    <div className="col fill" style={{ gap: 4, minHeight: 0 }}>
      <div className="row" style={{ minHeight: 28, flexWrap: 'wrap', rowGap: 2 }}>
        {isBible ? (
          <>
            <label className="chk" style={{ fontWeight: 600, whiteSpace: 'nowrap' }}>
              <input type="checkbox" checked={multi} onChange={(e) => toggleMulti(e.target.checked)} />
              {t('Multi-Verse Mode')}
            </label>
            <span className="grow" />
            {/* the version buttons float over the preview box (see LivePreview); without the box they stay here */}
            {!settings.app.livePreview && <><BibleVersionButtons /><span className="grow" /></>}
          </>
        ) : (
          <>
            <span className="grow" />
          </>
        )}
        <PresentationButtons show={ui.tab === 'bible' && multi} canPreview={dirty && pending.length > 0} holdPreview={holdPreview} canUpdate={dirty} onUpdate={update} />
      </div>
      {settings.app.livePreview && <LivePreview override={peek} hint={hint} overlay={isBible || ui.tab === 'bible' ? <BibleVersionButtons /> : undefined} />}
      {live.notes && (
        <div className="muted" style={{ whiteSpace: 'pre-wrap', fontSize: 12 }}>
          {t('Notes:')}
          {'\n'}
          {live.notes}
        </div>
      )}
      {live.type === 'video' ? (
        <VideoControls />
      ) : (
        <div ref={listRef} className={`list grow active ${quiet ? 'quiet' : ''} ${live.type === 'picture' ? 'thumb-list live-thumbs' : 'live-list hover-items'}`} tabIndex={0} onKeyDown={onListKey} onKeyUp={(e) => { if (e.key === ' ') holdPreview(false, 'key'); }} onBlur={() => { if (holdSource.current === 'key') holdPreview(false); }} style={{ background: 'var(--live-bg)' }}>
          {live.items.map((it, i) => (
            <div
              key={i}
              className={live.type === 'picture' ? `thumb ${live.selection.includes(i) ? 'sel' : ''}` : `item spaced ${isBible ? 'verse' : ''} ${(isBible && multi ? pending : live.selection).includes(i) ? 'sel' : ''} ${live.type === 'song' ? 'nowrap' : ''}`}
              onMouseDown={(e) => onDown(i, e)} onMouseEnter={() => onEnter(i)}
              onDoubleClick={() => { if (!(isBible && multi)) api.liveIndex(i, true); }}
            >
              {it.image ? <img src={it.image} alt="" draggable={false} /> : isBible ? <BibleLine text={it.text} /> : live.type === 'song' ? <StanzaText text={it.text} /> : it.text}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

/** `overlay` is shown at the bottom, in the middle, while the pointer is over the box (or something in it has the focus). */
export function LivePreview({ override, overlay, hint = 0 }: { override?: RenderSlide | null; overlay?: React.ReactNode; hint?: number }) {
  const { live } = useStore();
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
  // the preview has the shape of the screen it stands for, so its layout (line breaks, fitting) is the same one
  // the display shows; 16:9 when the screen size is not known
  const aspect = d?.width && d?.height ? Math.max(1, Math.min(4, d.width / d.height)) : 16 / 9;
  const slide = useMemo(() => override ?? d?.preview ?? d?.slide, [d, override]);
  return (
    <div ref={ref} className={`live-preview ${hint === 1 ? 'hint' : hint === 2 ? 'hint-fade' : ''}`} style={{ width: '100%', aspectRatio: String(aspect), position: 'relative', overflow: 'hidden', background: '#000', border: '1px solid var(--border)', flex: 'none', boxSizing: 'border-box' }}>
      {slide && (
        <div style={{ position: 'absolute', inset: 0, pointerEvents: 'none' }}>
          {slide.kind === 'video' ? (
            <div style={{ color: '#fff', padding: 12, font: `${Math.max(10, w / 28)}px sans-serif` }}>{slide.video?.name}</div>
          ) : (
            <ScaledSlide slide={slide} width={w} height={w / aspect} />
          )}
        </div>
      )}
      {overlay && <div className="lp-overlay"><div className="lp-overlay-in">{overlay}</div></div>}
    </div>
  );
}

function fmtTime(s: number) {
  if (!isFinite(s) || s < 0) s = 0;
  const m = Math.floor(s / 60);
  const sec = Math.floor(s % 60);
  return `${m}:${sec.toString().padStart(2, '0')}`;
}

export function VideoControls() {
  const { live } = useStore();
  const v = live.video;
  return (
    <div className="col" style={{ padding: 6, border: '1px solid var(--border-light)', borderRadius: 4 }}>
      <div className="row">
        <button className="icon-btn" title={t('Play')} onClick={() => api.video('play')} disabled={v.playing}>
          <Icon name="play.png" size={20} />
        </button>
        <button className="icon-btn" title={t('Pause')} onClick={() => api.video('pause')} disabled={!v.playing}>
          <Icon name="pause.png" size={20} />
        </button>
        <button className="icon-btn" title={t('Stop')} onClick={() => api.video('stop')}>
          <Icon name="stop.png" size={20} />
        </button>
        <span className="muted" style={{ marginLeft: 'auto' }}>
          {fmtTime(v.position)} / {fmtTime(v.duration)}
        </span>
      </div>
      <input
        type="range"
        min={0}
        max={Math.max(1, v.duration)}
        step={0.1}
        value={Math.min(v.position, v.duration || 0)}
        onChange={(e) => api.video('seek', Number(e.target.value))}
      />
      <div className="row">
        <button className="icon-btn" onClick={() => api.video('mute', v.muted ? 0 : 1)} title={t('Mute')}>
          <Icon name={v.muted ? 'speakerMute.png' : 'speaker.png'} size={18} />
        </button>
        <input type="range" min={0} max={100} value={v.volume} onChange={(e) => api.video('volume', Number(e.target.value))} style={{ flex: 1 }} />
      </div>
    </div>
  );
}

/** "12. verse text" with the number in its own column so wrapped lines align with the text. */
function BibleLine({ text }: { text: string }) {
  const m = text.match(/^(\d+\.)\s([\s\S]*)$/);
  if (!m) return <>{text}</>;
  return (
    <>
      <span className="verse-num">{m[1]}</span>
      <span className="verse-text">{m[2]}</span>
    </>
  );
}

/** Play / Clear: control what the target displays show. Lives at the right edge of the live header (Off is in the top bar). */
function PresentationButtons({ show, canPreview, holdPreview, canUpdate, onUpdate }: { show: boolean; canPreview: boolean; holdPreview: (on: boolean, source?: 'pointer' | 'key') => void; canUpdate: boolean; onUpdate: () => void }) {
  const { live } = useStore();
  // Play is only available when it would change what the presenter windows show now:
  // the windows are closed (Play opens them), they show the passive screen although there is content, or the content was cleared
  const windowsOpen = live.singleScreen ? live.showing : live.displayOn;
  const playChangesDisplays = !!live.type && (!windowsOpen || !live.showing || live.cleared);
  const Btn = ({ title, onClick, disabled, active, children }: { title: string; onClick: () => void; disabled?: boolean; active?: boolean; children: React.ReactNode }) => (
    <button className={`tbtn ${active ? 'checked' : ''}`} title={title} aria-label={title} aria-pressed={active} onClick={onClick} disabled={disabled}>
      <svg viewBox="0 0 24 24" aria-hidden="true">{children}</svg>
    </button>
  );
  return (
    <span className="toolbar present-btns">
      {/* hold to see the new selection in the preview box; release to go back (momentary) */}
      <button
        className="tbtn"
        title={t('Hold to preview the new selection')}
        aria-label={t('Preview')}
        // only shown on the Bible tab in multi-verse mode, but it always keeps its place so Play and Clear don't move
        style={show ? undefined : { visibility: 'hidden' }}
        tabIndex={show ? 0 : -1}
        aria-hidden={!show}
        disabled={!canPreview}
        onPointerDown={(e) => { try { e.currentTarget.setPointerCapture(e.pointerId); } catch { /* not an active pointer */ } holdPreview(true); }}
        onPointerUp={() => holdPreview(false)}
        onPointerCancel={() => holdPreview(false)}
        onKeyDown={(e) => { if ((e.key === ' ' || e.key === 'Enter') && !e.repeat) { e.preventDefault(); holdPreview(true, 'key'); } }}
        onKeyUp={(e) => { if (e.key === ' ' || e.key === 'Enter') holdPreview(false, 'key'); }}
        onBlur={() => holdPreview(false)}
      >
        <svg viewBox="0 0 24 24" aria-hidden="true">
          <path d="M7 16m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0" /><path d="M17 16m-3 0a3 3 0 1 0 6 0a3 3 0 1 0 -6 0" />
          <path d="M16.346 9.17l-.729 -1.261c-.16 -.248 -1.056 -.203 -1.117 .091l-.177 1.38" />
          <path d="M19.761 14.813l-2.84 -5.133c-.189 -.31 -.592 -.68 -1.421 -.68c-.828 0 -1.5 .448 -1.5 1v6" />
          <path d="M7.654 9.17l.729 -1.261c.16 -.249 1.056 -.203 1.117 .091l.177 1.38" />
          <path d="M4.239 14.813l2.84 -5.133c.189 -.31 .592 -.68 1.421 -.68c.828 0 1.5 .448 1.5 1v6" />
          <rect width="4" height="2" x="10" y="12" />
        </svg>
      </button>
      <Btn title={t('Clear')} onClick={() => api.clear()} disabled={!live.type}><path d="M19 20h-10.5l-4.21 -4.3a1 1 0 0 1 0 -1.41l10 -10a1 1 0 0 1 1.41 0l5 5a1 1 0 0 1 0 1.41l-9.2 9.3" /><path d="M18 13.3l-6.3 -6.3" /></Btn>
      {/* Pause blanks the displays until Play (or a double-clicked item) brings the content back; it stays pressed meanwhile and does not undo itself */}
      <Btn title={live.cleared ? t('Paused: press Play to resume') : t('Pause')} onClick={() => api.pause()} disabled={!live.type || live.cleared} active={live.cleared}><path d="M6 5h4v14H6z" /><path d="M14 5h4v14h-4z" /></Btn>
      <Btn title={canUpdate ? t('Show the new selection') : t('Play')} onClick={() => (canUpdate ? onUpdate() : api.show())} disabled={!canUpdate && !playChangesDisplays}><path d="M7 4v16l13 -8z" /></Btn>
    </span>
  );
}
