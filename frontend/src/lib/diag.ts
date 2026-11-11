// Reports of slow moments from the pages to the log of the program (POST /api/clientlog), so that a delay seen on one
// computer can be told apart: a request that took long, a state that came late, a page that was busy.
let sentAt: number[] = [];

export function report(text: string) {
  const now = Date.now();
  sentAt = sentAt.filter((t) => now - t < 60000);
  if (sentAt.length >= 20) return; // a page that is slow all the time must not fill the log
  sentAt.push(now);
  const who = location.pathname.startsWith('/projector') ? 'display page' : 'operator page';
  try {
    fetch('/api/clientlog', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ text: `${who}: ${text}` }), keepalive: true }).catch(() => {});
  } catch {
    /* the log is only for finding delays */
  }
}

let lastLive: { path: string; t: number } | null = null;

/** A request that runs the presentation was sent. */
export function liveSent(path: string) {
  lastLive = { path, t: performance.now() };
}

/** A new state of the presentation arrived (from the program, by the websocket). */
export function stateArrived() {
  if (!lastLive) return;
  const d = performance.now() - lastLive.t;
  if (d < 15000 && d > 400) report(`the state came ${Math.round(d)} ms after ${lastLive.path} was sent`);
  lastLive = null;
}

/** Writes down every stretch of 300 ms or more in which the page did nothing else (a frozen page reacts late to everything). */
export function watchLongTasks() {
  try {
    new PerformanceObserver((list) => {
      for (const e of list.getEntries()) if (e.duration >= 300) report(`the page was busy for ${Math.round(e.duration)} ms`);
    }).observe({ entryTypes: ['longtask'] });
  } catch {
    /* not every web view has long task timing */
  }
}
