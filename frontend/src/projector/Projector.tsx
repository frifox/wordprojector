import { useEffect, useRef, useState } from 'react';
import Socket, { type WsMessage } from '../lib/ws';
import type { LiveState, RenderSlide } from '../lib/types';
import ScaledSlide from './ScaledSlide';

/** Full screen projector page: /projector/N */
export default function Projector({ disp }: { disp: number }) {
  const [state, setState] = useState<LiveState | null>(null);
  const [size, setSize] = useState({ w: window.innerWidth, h: window.innerHeight });
  const socketRef = useRef<Socket | null>(null);
  const videoRef = useRef<HTMLVideoElement>(null);

  useEffect(() => {
    const s = new Socket('projector', disp);
    socketRef.current = s;
    const off = s.on((m: WsMessage) => {
      if (m.type === 'state') setState(m.state);
      else if (m.type === 'video') {
        const v = videoRef.current;
        if (!v) return;
        switch (m.cmd) {
          case 'play':
            v.play().catch(() => {});
            break;
          case 'pause':
            v.pause();
            break;
          case 'stop':
            v.pause();
            v.currentTime = 0;
            break;
          case 'seek':
            v.currentTime = m.value || 0;
            break;
          case 'volume':
            v.volume = Math.max(0, Math.min(1, (m.value || 0) / 100));
            break;
          case 'mute':
            v.muted = !!m.value || disp > 1;
            break;
        }
      }
    });
    s.start();
    const onResize = () => setSize({ w: window.innerWidth, h: window.innerHeight });
    window.addEventListener('resize', onResize);
    const onKey = (e: KeyboardEvent) => {
      if (['ArrowRight', 'ArrowDown', 'PageDown', 'Enter', ' '].includes(e.key)) s.send({ type: 'next' });
      else if (['ArrowLeft', 'ArrowUp', 'PageUp', 'Backspace'].includes(e.key)) s.send({ type: 'prev' });
      else if (e.key === 'Escape') s.send({ type: 'exit' });
      else if (e.key === 'Delete') s.send({ type: 'stop' });
    };
    window.addEventListener('keydown', onKey);
    return () => {
      off();
      s.stop();
      window.removeEventListener('resize', onResize);
      window.removeEventListener('keydown', onKey);
    };
  }, [disp]);

  // Tell the program when this page has drawn its first state (the window is shown only then) and when it has drawn
  // (and finished fading to) each later one (a window is hidden only then, so it never keeps an old picture that
  // would flash the next time it is shown).
  const readySent = useRef(false);
  useEffect(() => {
    if (!state || readySent.current) return;
    readySent.current = true;
    window.setTimeout(() => socketRef.current?.send({ type: 'ready', disp }), 50);
  }, [state, disp]);
  // Tell the program when this page has drawn each state (the window is hidden only then, so it never keeps an old
  // picture that would flash the next time it is shown). Turning off is a cut, so this takes a couple of frames.
  const version = state?.version;
  const fadeMs = (() => {
    const sl = (state?.displays.find((d) => d.disp === disp) || state?.displays[0])?.slide;
    return sl?.fade ? sl.fadeMillis || 500 : 0;
  })();
  useEffect(() => {
    if (version === undefined) return;
    let raf2 = 0;
    let timer = 0;
    const raf1 = requestAnimationFrame(() => {
      raf2 = requestAnimationFrame(() => {
        timer = window.setTimeout(() => socketRef.current?.send({ type: 'settled', disp, version }), fadeMs + 10);
      });
    });
    return () => {
      cancelAnimationFrame(raf1);
      cancelAnimationFrame(raf2);
      window.clearTimeout(timer);
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [version]);

  const display = state?.displays.find((d) => d.disp === disp) || state?.displays[0];
  const slide = display?.slide;
  return (
    <div style={{ position: 'fixed', inset: 0, background: '#000', overflow: 'hidden', cursor: 'none' }}>
      {slide && <Stage slide={slide} width={size.w} height={size.h} videoRef={videoRef} socket={socketRef} disp={disp} state={state} />}
      {state?.controls.visible && disp === 1 && <Controls state={state} socket={socketRef} w={size.w} h={size.h} />}
    </div>
  );
}

/**
 * Cross fades between layers when the slide changes. The layers stay composited (will-change), so the text is drawn the same
 * way while it fades and after, instead of shifting a little when the fade ends.
 * Three layers take turns: every layer is always in the page (an
 * empty one at opacity 0), so a new slide is faded in from 0 instead of appearing at once on a layer that was only just
 * added, and a quick second change does not reuse a layer that is still fading out.
 */
function Stage({ slide, width, height, videoRef, socket, disp, state }: {
  slide: RenderSlide; width: number; height: number; videoRef: React.RefObject<HTMLVideoElement | null>;
  socket: React.RefObject<Socket | null>; disp: number; state: LiveState | null;
}) {
  // every slide is a layer of its own, stacked in order: a new one fades in over whatever is showing (however many
  // are still fading) and the layers under it are dropped once it is fully opaque
  type Layer = { id: number; slide: RenderSlide; fade: boolean; ms: number };
  const nextId = useRef(1);
  const timers = useRef<number[]>([]);
  const [layers, setLayers] = useState<Layer[]>([{ id: 0, slide, fade: false, ms: 500 }]);
  const lastKey = useRef(JSON.stringify(slide));
  useEffect(() => {
    const key = JSON.stringify(slide);
    if (key === lastKey.current) return;
    lastKey.current = key;
    // a hidden window can't run its fade: change instantly so nothing stale is left when it is shown again
    const layer: Layer = { id: nextId.current++, slide, fade: slide.fade && !document.hidden, ms: slide.fadeMillis || 500 };
    setLayers((l) => [...l, layer]);
    const id = window.setTimeout(() => setLayers((l) => l.filter((x) => x.id >= layer.id)), layer.fade ? layer.ms + 30 : 0);
    timers.current.push(id);
  }, [slide]);
  useEffect(() => () => timers.current.forEach((id) => window.clearTimeout(id)), []);
  // when the window becomes visible again, show the current slide at once (no fade from whatever it showed before)
  useEffect(() => {
    const snap = () => {
      if (document.hidden) return;
      lastKey.current = JSON.stringify(slide);
      timers.current.forEach((id) => window.clearTimeout(id));
      setLayers([{ id: nextId.current++, slide, fade: false, ms: 500 }]);
    };
    document.addEventListener('visibilitychange', snap);
    return () => document.removeEventListener('visibilitychange', snap);
  }, [slide]);
  const render = (layer: Layer, visible: boolean) => { const s = layer.slide; return (
      <div key={layer.id} style={{ position: 'absolute', inset: 0, animation: layer.fade ? `wp-fade-in ${layer.ms / 1000}s ease both` : 'none', pointerEvents: 'none', willChange: 'opacity' }}>
        {!s ? null : s.kind === 'video' && s.video ? (
          <video
            ref={visible ? videoRef : undefined}
            src={s.video.url}
            autoPlay
            muted={s.video.muted}
            style={{ position: 'absolute', inset: 0, width: '100%', height: '100%', objectFit: 'contain', background: '#000' }}
            onTimeUpdate={(e) => {
              if (disp !== 1) return;
              const v = e.currentTarget;
              socket.current?.send({ type: 'videoStatus', disp, position: v.currentTime, duration: v.duration || 0, playing: !v.paused && !v.ended, ended: v.ended });
            }}
            onEnded={(e) => {
              const v = e.currentTarget;
              socket.current?.send({ type: 'videoStatus', disp, position: v.currentTime, duration: v.duration || 0, playing: false, ended: true });
            }}
            onPlay={(e) => {
              const v = e.currentTarget;
              if (state && state.video.volume >= 0) v.volume = state.video.volume / 100;
              socket.current?.send({ type: 'videoStatus', disp, position: v.currentTime, duration: v.duration || 0, playing: true, ended: false });
            }}
            onPause={(e) => {
              const v = e.currentTarget;
              socket.current?.send({ type: 'videoStatus', disp, position: v.currentTime, duration: v.duration || 0, playing: false, ended: v.ended });
            }}
          />
        ) : (
          <ScaledSlide slide={s} width={width} height={height} />
        )}
      </div>
    ); };
  return (
    <>
      {layers.map((l, i) => render(l, i === layers.length - 1))}
    </>
  );
}

function Controls({ state, socket, w, h }: { state: LiveState; socket: React.RefObject<Socket | null>; w: number; h: number }) {
  const c = state.controls;
  const sizes = [16, 24, 32, 48, 64, 96];
  const size = sizes[c.buttonSize] ?? 48;
  const margin = 20;
  const total = size * 3 + 20;
  const y = c.alignV === 0 ? margin : c.alignV === 1 ? (h - size) / 2 : h - size - margin;
  const x = c.alignH === 0 ? margin : c.alignH === 1 ? (w - total) / 2 : w - total - margin;
  const [hover, setHover] = useState(false);
  const btn = (name: string, onClick: () => void) => (
    <img
      src={`/icons/control${name}.png`}
      alt={name}
      width={size}
      height={size}
      style={{ cursor: 'pointer', pointerEvents: 'auto' }}
      onMouseDown={(e) => {
        e.preventDefault();
        onClick();
      }}
    />
  );
  return (
    <div
      style={{ position: 'absolute', left: x, top: y, display: 'flex', gap: 10, opacity: hover ? 1 : c.opacity, transition: 'opacity 0.2s', cursor: 'default' }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => setHover(false)}
    >
      {btn('Prev', () => socket.current?.send({ type: 'prev' }))}
      {btn('Next', () => socket.current?.send({ type: 'next' }))}
      {btn('Exit', () => socket.current?.send({ type: 'exit' }))}
    </div>
  );
}
