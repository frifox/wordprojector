import { useState } from 'react';
import Modal, { useDiscardGuard } from '../components/Modal';
import { t } from '../i18n';

/** Generic "name + information" dialog used for songbooks and themes. */
export default function NameInfoDialog({ title, nameLabel, infoLabel, name = '', info = '', onSave, onClose }: {
  /** without infoLabel only the name is asked for */
  title: string; nameLabel: string; infoLabel?: string; name?: string; info?: string;
  onSave: (name: string, info: string) => void; onClose: () => void;
}) {
  const [n, setN] = useState(name);
  const [i, setI] = useState(info);
  const requestClose = useDiscardGuard(n !== name || i !== info, onClose, () => ok());
  const ok = () => {
    if (!n.trim()) return;
    onSave(n.trim(), i.trim());
  };
  return (
    <Modal title={title} onClose={requestClose} width={420} buttons={<><button onClick={requestClose}>{t('Cancel')}</button><button className="primary" onClick={ok} disabled={!n.trim()}>{t('OK')}</button></>}>
      <div className="form-grid">
        <span>{nameLabel}</span>
        <input type="text" value={n} autoFocus onChange={(e) => setN(e.target.value)} onKeyDown={(e) => e.key === 'Enter' && ok()} />
        {infoLabel !== undefined && <>
          <span style={{ alignSelf: 'flex-start' }}>{infoLabel}</span>
          <textarea rows={5} value={i} onChange={(e) => setI(e.target.value)} />
        </>}
      </div>
    </Modal>
  );
}
