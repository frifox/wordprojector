/**
 * Vertical scrollbars float over the content (the real ones are made 0 wide in index.css): every scrolling area gets a thin
 * thumb at its right edge, always shown while there is something to scroll, and it can be dragged.
 */
/** the space between the thumb and the edges of the area: the same on the right, the top and the bottom */
const GAP = 2;
const CANDIDATES = 'div, textarea, ul, ol, pre, section, main, aside';

interface Bar {
  el: HTMLElement;
  thumb: HTMLDivElement;
  ro: ResizeObserver;
  /** the strip at the top the thumb stays below (the column headers of a table do not scroll) */
  inset: number;
}

export function setupOverlayScroll() {
  const bars = new Map<HTMLElement, Bar>();
  let queued = false;

  const isScroller = (e: HTMLElement) => {
    const o = getComputedStyle(e).overflowY;
    return o === 'auto' || o === 'scroll';
  };

  const drop = (b: Bar) => {
    b.ro.disconnect();
    b.thumb.remove();
    bars.delete(b.el);
  };

  const layout = (b: Bar) => {
    const { el, thumb } = b;
    const room0 = el.clientHeight;
    if (!el.isConnected) { drop(b); return; }
    if (room0 === 0 || el.scrollHeight <= room0 + 1) { thumb.style.display = 'none'; return; }
    const r = el.getBoundingClientRect();
    b.inset = el.querySelector(':scope > table > thead')?.getBoundingClientRect().height ?? 0;
    const track = room0 - b.inset - 2 * GAP;
    const h = Math.max(28, (track * room0) / el.scrollHeight);
    const room = track - h;
    const top = r.top + el.clientTop + b.inset + GAP + (room * el.scrollTop) / (el.scrollHeight - room0);
    const left = r.left + el.clientLeft + el.clientWidth - 7 - GAP;
    // a dialog or menu over the area hides its thumb
    const hit = document.elementsFromPoint(left + 3, r.top + el.clientTop + b.inset + GAP + track / 2).find((n) => n !== thumb && !n.classList.contains('ov-thumb'));
    if (!hit || !el.contains(hit)) { thumb.style.display = 'none'; return; }
    thumb.style.display = '';
    thumb.style.height = `${h}px`;
    thumb.style.top = `${top}px`;
    thumb.style.left = `${left}px`;
  };

  const layoutAll = () => {
    queued = false;
    for (const b of [...bars.values()]) layout(b);
  };
  const schedule = () => {
    if (queued) return;
    queued = true;
    requestAnimationFrame(layoutAll);
  };

  const add = (el: HTMLElement) => {
    if (bars.has(el) || el === document.body || el === document.documentElement || !isScroller(el)) return;
    const thumb = document.createElement('div');
    thumb.className = 'ov-thumb';
    thumb.style.display = 'none';
    document.body.appendChild(thumb);
    const ro = new ResizeObserver(schedule);
    ro.observe(el);
    const b: Bar = { el, thumb, ro, inset: 0 };
    bars.set(el, b);
    thumb.addEventListener('pointerdown', (e) => {
      e.preventDefault();
      thumb.classList.add('drag');
      const startY = e.clientY;
      const startTop = el.scrollTop;
      const move = (ev: PointerEvent) => {
        const room = el.clientHeight - b.inset - 2 * GAP - thumb.offsetHeight;
        if (room > 0) el.scrollTop = startTop + ((ev.clientY - startY) * (el.scrollHeight - el.clientHeight)) / room;
      };
      const up = () => {
        thumb.classList.remove('drag');
        window.removeEventListener('pointermove', move);
        window.removeEventListener('pointerup', up);
      };
      window.addEventListener('pointermove', move);
      window.addEventListener('pointerup', up);
    });
    layout(b);
  };

  const scan = (n: Node) => {
    if (!(n instanceof HTMLElement)) return;
    if (n.matches(CANDIDATES)) add(n);
    n.querySelectorAll<HTMLElement>(CANDIDATES).forEach(add);
  };

  scan(document.body);
  new MutationObserver((list) => {
    for (const m of list) m.addedNodes.forEach(scan);
    schedule();
  }).observe(document.body, { childList: true, subtree: true });
  // the content of an area grows and shrinks without the area itself changing size
  document.addEventListener('scroll', schedule, true);
  window.addEventListener('resize', schedule);
  // areas moved by their neighbours (a splitter being dragged, a panel opening) tell nobody
  window.setInterval(schedule, 150);
}
