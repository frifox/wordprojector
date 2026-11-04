import { useState } from 'react';
import { alertDialog } from '../components/Modal';
import { t } from '../i18n';
import type { InstallInfo, Settings } from '../lib/types';
import { InstallForm, UninstallForm, defaultUninstall, initialChoice, runInstall } from './InstallForm';
import { runUninstall } from './UninstallDialog';

/** Settings > Install: the install form, or, once the program is installed, what to take away. */
export default function InstallPage({ info, s, setS }: { info: InstallInfo; s: Settings; setS: (f: (x: Settings) => Settings) => void }) {
  const [c, setC] = useState(() => initialChoice(info, !!s.app.skipInstallPrompt));
  const [o, setO] = useState(defaultUninstall);
  const [busy, setBusy] = useState(false);

  if (info.installed) {
    const go = async () => {
      setBusy(true);
      if (!(await runUninstall(o))) setBusy(false);
    };
    return (
      <div className="col" style={{ gap: 10 }}>
        <UninstallForm info={info} value={o} onChange={setO} />
        <div className="row">
          <span className="grow" />
          {busy && <span className="muted">{t('Word Projector closes and removes what you chose.')}</span>}
          <button className="primary" disabled={busy || !(o.app || o.shortcuts || o.data || o.docs)} onClick={go}>{t('Uninstall')}</button>
        </div>
      </div>
    );
  }
  const install = async () => {
    setBusy(true);
    try {
      await runInstall(c);
    } catch (e) {
      alertDialog(t('The installation did not work'), String((e as Error).message || e));
      setBusy(false);
    }
  };
  return (
    <div className="col" style={{ gap: 10 }}>
      {!info.canInstall && <div className="muted">{t(info.why || '')}</div>}
      <InstallForm
        info={info}
        value={c}
        // the box for the offer at start is a setting like the others here: it is saved with OK
        onChange={(v) => { setC(v); if (v.skipAsk !== c.skipAsk) setS((x) => ({ ...x, app: { ...x.app, skipInstallPrompt: v.skipAsk } })); }}
      />
      <div className="row">
        <span className="grow" />
        {busy && <span className="muted">{t('Word Projector starts again from its new place.')}</span>}
        <button className="primary" disabled={busy || !c.install || !info.canInstall} onClick={install}>{t('Install')}</button>
      </div>
    </div>
  );
}
