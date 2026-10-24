import { useStore } from '../lib/store';
import Modal from '../components/Modal';
import { t } from '../i18n';

/** Shows running import/download jobs reported over the websocket. */
export default function ProgressDialog() {
  const { progress, clearProgress } = useStore();
  // the pictures of a folder being prepared have their own bar in the dialog that asked for them
  const jobs = Object.values(progress).filter((j) => j.job !== 'folder-images');
  if (!jobs.length) return null;
  const p = jobs[0];
  const pct = p.max && p.max > 0 ? Math.min(100, Math.round(((p.cur || 0) * 100) / p.max)) : p.done ? 100 : 0;
  const tpct = p.totalMax ? Math.min(100, Math.round(((p.total || 0) * 100) / p.totalMax)) : p.done ? 100 : 0;
  const titles: Record<string, string> = { 'import-songbook': t('Importing Songbook'), 'import-bible': t('Importing Bible'), 'import-theme': t('Importing Theme'), 'download-bible': t('Bible Module Download'), 'download-songbook': t('Songbook Module Download'), 'download-theme': t('Theme Module Download') };
  return (
    <Modal top title={titles[p.job] || p.job} width={520} closeOnEscape={false} buttons={<button className="primary" disabled={!p.done} onClick={() => clearProgress(p.job)}>{t('Close')}</button>}>
      <div className="col" style={{ gap: 6 }}>
        <div className="row"><span style={{ width: 70 }}>{t('Current:')}</span><progress value={pct} max={100} style={{ flex: 1 }} /><span className="muted" style={{ width: 90, textAlign: 'right' }}>{p.msg || ''}</span></div>
        {p.totalMax ? <div className="row"><span style={{ width: 70 }}>{t('Total:')}</span><progress value={tpct} max={100} style={{ flex: 1 }} /></div> : null}
        <pre style={{ background: 'var(--base)', border: '1px solid var(--border-light)', height: 160, overflow: 'auto', margin: 0, padding: 6, whiteSpace: 'pre-wrap', fontSize: 12 }}>{p.log.join('\n')}</pre>
        {p.done && !p.error && <div>{t('Done.')}</div>}
        {p.error && <div style={{ color: '#c33' }}>{p.error}</div>}
      </div>
    </Modal>
  );
}
