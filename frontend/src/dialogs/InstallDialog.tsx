import { useState } from 'react';
import Modal, { alertDialog } from '../components/Modal';
import { api } from '../lib/api';
import { useStore } from '../lib/store';
import { t } from '../i18n';
import type { InstallInfo } from '../lib/types';
import { InstallForm, initialChoice, runInstall } from './InstallForm';

/** Shown when the program starts from somewhere it was not installed to: keep it there, or install it. */
export default function InstallDialog({ info, onClose }: { info: InstallInfo; onClose: () => void }) {
  const store = useStore();
  const [c, setC] = useState(() => initialChoice(info, true));
  const [busy, setBusy] = useState(false);
  const ok = async () => {
    setBusy(true);
    try {
      if (c.install) {
        await runInstall(c); // the program restarts from the new place
        return;
      }
      await api.patchApp({ skipInstallPrompt: c.skipAsk });
      await store.reloadSettings();
      onClose();
    } catch (e) {
      alertDialog(t('The installation did not work'), String((e as Error).message || e));
      setBusy(false);
    }
  };
  return (
    <Modal title={t('Install Word Projector?')} onClose={busy ? () => {} : onClose} width={520}
      buttons={
        <>
          <button disabled={busy} onClick={onClose}>{t('Cancel')}</button>
          <button className="primary" disabled={busy} onClick={ok}>{busy && c.install ? t('Installing…') : c.install ? t('Install') : t('OK')}</button>
        </>
      }>
      <InstallForm info={info} value={c} onChange={setC} />
      {busy && c.install && <div className="muted" style={{ marginTop: 10 }}>{t('Word Projector starts again from its new place.')}</div>}
    </Modal>
  );
}
