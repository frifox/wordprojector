import { useState } from 'react';
import Modal, { alertDialog, confirmDialog } from '../components/Modal';
import { api } from '../lib/api';
import { t } from '../i18n';
import type { InstallInfo } from '../lib/types';
import { UninstallForm, defaultUninstall } from './InstallForm';

/** Asks and then does the uninstall: Word Projector quits and takes away what was chosen. */
export async function runUninstall(o: typeof defaultUninstall): Promise<boolean> {
  if (o.data || o.docs) {
    const go = await confirmDialog({
      title: t('Delete your data?'),
      message: t('Your songs, settings and downloaded books (the data folder), or your slide show pictures (the documents folder), will be deleted for good.'),
      buttons: [{ label: t('Uninstall'), value: 'yes', primary: true }, { label: t('Cancel'), value: 'no' }],
    });
    if (go !== 'yes') return false;
  }
  try {
    await api.uninstall(o);
    return true;
  } catch (e) {
    alertDialog(t('The uninstall did not work'), String((e as Error).message || e));
    return false;
  }
}

/** Shown when the program is started from the uninstall entry of the system. */
export default function UninstallDialog({ info, onClose }: { info: InstallInfo; onClose: () => void }) {
  const [o, setO] = useState(defaultUninstall);
  const [busy, setBusy] = useState(false);
  const go = async () => {
    setBusy(true);
    if (!(await runUninstall(o))) setBusy(false);
  };
  return (
    <Modal title={t('Uninstall Word Projector')} onClose={busy ? () => {} : onClose} width={520}
      buttons={
        <>
          <button disabled={busy} onClick={onClose}>{t('Cancel')}</button>
          <button className="primary" disabled={busy || !(o.app || o.shortcuts || o.data || o.docs)} onClick={go}>{t('Uninstall')}</button>
        </>
      }>
      <UninstallForm info={info} value={o} onChange={setO} />
      {busy && <div className="muted" style={{ marginTop: 10 }}>{t('Word Projector closes and removes what you chose.')}</div>}
    </Modal>
  );
}
