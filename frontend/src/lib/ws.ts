import type { LiveState, ProgressMsg, ScheduleState, VideoState } from './types';

export type WsMessage =
  | { type: 'state'; state: LiveState }
  | ({ type: 'schedule' } & ScheduleState)
  | { type: 'data'; what: string }
  | { type: 'video'; cmd: string; value?: number }
  | { type: 'videoStatus'; video: VideoState }
  | ProgressMsg;

type Listener = (m: WsMessage) => void;

class Socket {
  private ws: WebSocket | null = null;
  private listeners = new Set<Listener>();
  private role: string;
  private disp: number;
  private timer: number | null = null;
  connected = false;
  onStatus: ((c: boolean) => void) | null = null;

  constructor(role: string, disp = 0) {
    this.role = role;
    this.disp = disp;
  }

  start() {
    this.connect();
  }

  private connect() {
    const proto = location.protocol === 'https:' ? 'wss' : 'ws';
    const ws = new WebSocket(`${proto}://${location.host}/api/ws`);
    this.ws = ws;
    ws.onopen = () => {
      this.connected = true;
      this.onStatus?.(true);
      ws.send(JSON.stringify({ type: 'hello', role: this.role, disp: this.disp }));
    };
    ws.onmessage = (ev) => {
      try {
        const m = JSON.parse(ev.data) as WsMessage;
        this.listeners.forEach((l) => l(m));
      } catch {
        /* ignore */
      }
    };
    ws.onclose = () => {
      this.connected = false;
      this.onStatus?.(false);
      this.ws = null;
      this.timer = window.setTimeout(() => this.connect(), 1000);
    };
    ws.onerror = () => ws.close();
  }

  send(obj: unknown) {
    if (this.ws && this.ws.readyState === WebSocket.OPEN) this.ws.send(JSON.stringify(obj));
  }

  on(l: Listener) {
    this.listeners.add(l);
    return () => {
      this.listeners.delete(l);
    };
  }

  stop() {
    if (this.timer) window.clearTimeout(this.timer);
    this.ws?.close();
  }
}

export default Socket;
