import { useState } from 'react';
import Modal, { alertDialog } from '../components/Modal';
import { t } from '../i18n';
import { useStore } from '../lib/store';
import { usePickerActive } from '../lib/picker';
import { pickFolder, type FolderPick } from '../lib/files';

/** New slide show from a folder: pick the folder (native dialog) and name the show. */
export default function FolderShowDialog({ onCreate, onClose }: {
  onCreate: (folder: FolderPick, name: string) => Promise<boolean>; onClose: () => void;
}) {
  const { boot } = useStore();
  const [folder, setFolder] = useState<FolderPick | null>(null);
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const picking = usePickerActive();
  const browse = async () => {
    const f = await pickFolder(boot.desktop, t('Select Folder with Images'));
    if (!f) return;
    setFolder(f);
    if (!touched && f.name) setName(f.name);
  };
  const ok = async () => {
    if (!folder || !name.trim() || busy) return;
    setBusy(true);
    const done = await onCreate(folder, name.trim());
    setBusy(false);
    if (!done) alertDialog(t('No images found'), t('The selected folder has no images.'), t('Please select another folder.'));
  };
  return (
    <Modal title={t('New Slide Show from Folder')} onClose={onClose} width={460} buttons={<><button onClick={onClose}>{t('Cancel')}</button><button className="primary" onClick={ok} disabled={!folder || !name.trim() || busy}>{busy ? t('Importing...') : t('OK')}</button></>}>
      <div className="form-grid">
        <span>{t('Folder:')}</span>
        <div className="row" style={{ gap: 6 }}>
          <input type="text" className="grow" readOnly value={folder ? folder.dir ?? folder.name : ''} placeholder={t('(none selected)')} title={folder?.dir} />
          <button onClick={browse} disabled={picking} autoFocus>{t('Browse...')}</button>
        </div>
        <span>{t('Title:')}</span>
        <input type="text" value={name} onChange={(e) => { setName(e.target.value); setTouched(true); }} onKeyDown={(e) => e.key === 'Enter' && ok()} />
      </div>
    </Modal>
  );
}
