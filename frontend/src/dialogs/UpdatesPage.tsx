import { useEffect, useState } from 'react';
import { alertDialog, confirmDialog } from '../components/Modal';
import { api } from '../lib/api';
import { useData } from '../lib/hooks';
import { t } from '../i18n';
import type { Settings, UpdateRelease } from '../lib/types';

/** Releases are tagged with a day, "2026-10-22", and ".2" for a second one that day. A suffix like -3-gabc is ignored. */
function sameVersion(a: string, b: string): boolean {
  const key = (v: string) => {
    const m = /^(\d{4})-(\d{2})-(\d{2})(?:\.(\d+))?/.exec(v.trim());
    return m ? `${m[1]}-${m[2]}-${m[3]}.${Number(m[4] || 0)}` : null;
  };
  const x = key(a);
  const y = key(b);
  return x !== null && y !== null ? x === y : a === b;
}

/** Settings > Updates: the version in use, every release to pick from, and how updating happens. */
export default function UpdatesPage({ s, setS }: { s: Settings; setS: (f: (x: Settings) => Settings) => void }) {
  const [st, reload] = useData('update', () => api.updateStatus());
  const [picked, setPicked] = useState<string | null>(null);
  const [checking, setChecking] = useState(false);
  const [busy, setBusy] = useState(false);

  // the page opens with the latest release selected
  useEffect(() => {
    if (st && picked === null) setPicked(st.latest || st.current);
  }, [st, picked]);

  const check = async () => {
    setChecking(true);
    try {
      await api.updateStatus(true);
      reload();
    } finally {
      setChecking(false);
    }
  };

  // a first look when the page opens, if nothing has been fetched yet
  useEffect(() => {
    if (st && !st.checkedAt && !st.error) check();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [st?.checkedAt]);

  const app = s.app;
  const setApp = (p: Partial<Settings['app']>) => setS((x) => ({ ...x, app: { ...x.app, ...p } }));

  if (!st) return <div className="muted">{t('Loading…')}</div>;
  const rel: UpdateRelease | undefined = st.releases.find((r) => r.tag === picked);
  const installing = !!st.installing || busy;
  const canInstall = !!rel && rel.installable && st.supported && !sameVersion(rel.tag, st.current) && !installing;

  const install = async () => {
    if (!rel) return;
    const downgrade = st.releases.findIndex((r) => r.tag === rel.tag) > st.releases.findIndex((r) => sameVersion(r.tag, st.current)) && st.releases.some((r) => sameVersion(r.tag, st.current));
    const restart = st.canRestart;
    const go = await confirmDialog({
      title: downgrade ? t('Install an older version?') : t('Install {tag}?', { tag: rel.tag }),
      message: downgrade ? t('Install {tag}, which is older than the version in use?', { tag: rel.tag }) : t('Install version {tag}?', { tag: rel.tag }),
      info: restart ? t('Word Projector will close and open again. Anything being shown stops.') : t('It is used the next time Word Projector starts.'),
      buttons: [{ label: restart ? t('Install and Restart') : t('Install'), value: 'yes', primary: true }, { label: t('Cancel'), value: 'no' }],
    });
    if (go !== 'yes') return;
    setBusy(true);
    try {
      await api.installUpdate(rel.tag, restart);
      reload();
    } catch (e) {
      alertDialog(t('The update did not work'), String((e as Error).message || e));
    } finally {
      setBusy(false);
    }
  };

  const restartNow = async () => {
    try {
      await api.restartApp();
    } catch (e) {
      alertDialog(t('Error'), String((e as Error).message || e));
    }
  };

  const checkedAt = st.checkedAt ? new Date(st.checkedAt).toLocaleString() : '';
  return (
    <div className="col" style={{ gap: 8 }}>
      <fieldset className="group">
        <legend>{t('Updating')}</legend>
        <div className="col" style={{ gap: 4, alignItems: 'flex-start' }}>
        <label className="chk">
          <input type="checkbox" checked={!app.skipUpdateCheck} onChange={(e) => setApp({ skipUpdateCheck: !e.target.checked })} />
          {t('Check for updates every launch')}
        </label>
        <label className="chk" style={{ opacity: app.skipUpdateCheck ? 0.5 : 1 }} title={t('The newest release is installed by itself; it is used the next time Word Projector starts.')}>
          <input type="checkbox" checked={!!app.autoInstallUpdates} disabled={!!app.skipUpdateCheck} onChange={(e) => setApp({ autoInstallUpdates: e.target.checked })} />
          {t('Automatically Install Latest')}
        </label>
        </div>
      </fieldset>

      <fieldset className="group">
        <legend>{t('Releases')}</legend>
        <div>{t('Currently installed: {tag}', { tag: st.current })}</div>
        {st.pending && (
          <div className="row" style={{ marginTop: 6 }}>
            <span>{t('Version {tag} is installed. It starts the next time Word Projector is started.', { tag: st.pending })}</span>
            {st.canRestart && <button onClick={restartNow}>{t('Restart Now')}</button>}
          </div>
        )}
        {!st.supported && <div className="muted" style={{ marginTop: 6 }}>{t('Updating from here is not possible: {why}', { why: t(st.reason || '') })}</div>}
        <div className="row" style={{ margin: '6px 0' }}>
          <button onClick={check} disabled={checking || installing}>{checking ? t('Checking…') : t('Check Now')}</button>
          {checkedAt && <span className="muted">{t('Checked {when}', { when: checkedAt })}</span>}
          {st.error && <span style={{ color: 'var(--danger, #c0392b)' }}>{st.error}</span>}
        </div>
        <div className="list update-list" tabIndex={0} style={{ height: 170 }}>
          {st.releases.map((r) => (
            <div key={r.tag} className={`item spaced ${picked === r.tag ? 'sel' : ''}`} onClick={() => setPicked(r.tag)} style={{ opacity: r.installable ? 1 : 0.55 }}>
              <span className="update-tag">{r.tag}</span>
              <span className="update-name">{r.name && r.name !== r.tag ? r.name : ''}</span>
              <span className="update-marks">
                {r.tag === st.latest && <span className="update-chip">{t('Latest')}</span>}
                {sameVersion(r.tag, st.current) && <span className="update-chip on">{t('Installed')}</span>}
                {r.prerelease && <span className="update-chip">{t('Pre-release')}</span>}
                {!r.installable && <span className="update-chip">{t('No download for this computer')}</span>}
              </span>
              <span className="update-date">{r.published}</span>
            </div>
          ))}
          {!st.releases.length && <div className="item muted">{st.error ? t('The releases could not be fetched.') : t('No releases found.')}</div>}
        </div>
        {rel?.notes && <div className="update-notes">{rel.notes}</div>}
        <div className="row" style={{ marginTop: 6 }}>
          <span className="grow" />
          {installing && <span className="muted">{st.progress || t('Installing…')}</span>}
          {rel && !sameVersion(rel.tag, st.current) && (
            <button className="primary" disabled={!canInstall} onClick={install} title={!st.supported ? t(st.reason || '') : !rel.installable ? t('This release has no download for this computer') : undefined}>
              {t('Install Now')}
            </button>
          )}
        </div>
      </fieldset>

    </div>
  );
}
