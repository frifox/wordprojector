import { useEffect, useState } from 'react';
import Modal from '../components/Modal';
import { api } from '../lib/api';
import { t, getLanguage } from '../i18n';
import type { CountRow } from '../lib/types';

const months = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];

function fmtDate(d: string): string {
  const p = d.split(':');
  if (p.length !== 3) return d;
  const m = t(months[Number(p[0]) - 1] || '');
  return getLanguage() === 'en' ? `${m} ${p[1]}, ${p[2]}` : `${p[1]} ${m} ${p[2]}`;
}

export default function SongCounterDialog({ onClose }: { onClose: () => void }) {
  const [rows, setRows] = useState<CountRow[]>([]);
  const [sel, setSel] = useState<number>(0);
  const [sort, setSort] = useState<{ col: keyof CountRow; dir: 1 | -1 }>({ col: 'count', dir: -1 });
  const load = () => api.counts().then(setRows);
  useEffect(() => {
    load();
  }, []);
  const sorted = [...rows].sort((a, b) => {
    const av = a[sort.col];
    const bv = b[sort.col];
    return (typeof av === 'number' && typeof bv === 'number' ? av - bv : String(av).localeCompare(String(bv))) * sort.dir;
  });
  const cols: { key: keyof CountRow; label: string; w: number }[] = [
    { key: 'songbook', label: t('Songbook'), w: 150 },
    { key: 'number', label: t('Number'), w: 65 },
    { key: 'title', label: t('Title'), w: 0 },
    { key: 'count', label: t('Count'), w: 60 },
    { key: 'date', label: t('Date'), w: 140 },
  ];
  return (
    <Modal title={t('Song Counter')} onClose={onClose} width={720} height="70vh"
      buttons={<>
        <button onClick={async () => { await api.resetCounts(); load(); }}>{t('Reset All Counts')}</button>
        <button disabled={!sel} onClick={async () => { await api.resetCounts(sel); setSel(0); load(); }}>{t('Reset Selected')}</button>
        <span className="grow" />
        <button className="primary" onClick={onClose}>{t('Close')}</button>
      </>}>
      <div className="table grow" tabIndex={0}>
        <table>
          <colgroup>{cols.map((c) => <col key={c.key} style={c.w ? { width: c.w } : undefined} />)}</colgroup>
          <thead><tr>{cols.map((c) => <th key={c.key} onClick={() => setSort((s) => ({ col: c.key, dir: s.col === c.key ? ((s.dir * -1) as 1 | -1) : 1 }))}>{c.label}{sort.col === c.key ? (sort.dir === 1 ? ' ▲' : ' ▼') : ''}</th>)}</tr></thead>
          <tbody>
            {sorted.map((r) => (
              <tr key={r.id} className={r.id === sel ? 'sel' : ''} onClick={() => setSel(r.id)}>
                <td>{r.songbook}</td><td>{r.number}</td><td>{r.title}</td><td>{r.count}</td><td>{fmtDate(r.date)}</td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </Modal>
  );
}
