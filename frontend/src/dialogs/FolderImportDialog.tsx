import { useState } from 'react';
import Modal, { alertDialog } from '../components/Modal';
import { t } from '../i18n';
import { useStore } from '../lib/store';
import { usePickerActive } from '../lib/picker';
import { pickFolder, type FolderPick } from '../lib/files';

export interface FolderImportOptions {
  /** the pictures of the subfolders too */
  recursive: boolean;
  /** the slides keep a copy of each picture (false: they refer to the files where they lie) */
  copy: boolean;
}

/**
 * Slides from a folder, for a new slide show (mode "new": it also asks for the title) or to add to the open one: the
 * folder (native dialog), whether the subfolders count, whether the pictures are copied, and a bar while they are read.
 */
export default function FolderImportDialog({ mode, onImport, onClose }: {
  mode: 'new' | 'add';
  onImport: (folder: FolderPick, options: FolderImportOptions, name: string) => Promise<boolean>;
  onClose: () => void;
}) {
  const { boot, progress, clearProgress } = useStore();
  const [folder, setFolder] = useState<FolderPick | null>(null);
  const [name, setName] = useState('');
  const [touched, setTouched] = useState(false);
  const [busy, setBusy] = useState(false);
  const [options, setOptions] = useState<FolderImportOptions>({ recursive: false, copy: false }); // nothing is checked whenever the dialog opens
  const picking = usePickerActive();
  // in a browser the pictures are uploaded to the server anyway: there is nothing to link to
  const canLink = boot.desktop;
  const setOpt = (p: Partial<FolderImportOptions>) => setOptions((o) => ({ ...o, ...p }));
  const browse = async () => {
    const f = await pickFolder(boot.desktop, t('Select Folder with Images'));
    if (!f) return;
    setFolder(f);
    if (!touched && f.name) setName(f.name);
  };
  const needName = mode === 'new';
  const ready = !!folder && (!needName || !!name.trim()) && !busy;
  const ok = async () => {
    if (!ready || !folder) return;
    setBusy(true);
    clearProgress('folder-images');
    const done = await onImport(folder, { ...options, copy: options.copy || !canLink }, name.trim());
    setBusy(false);
    clearProgress('folder-images');
    if (!done) alertDialog(t('No images found'), t('The selected folder has no images.'), t('Please select another folder.'));
  };
  const p = progress['folder-images'];
  const pct = p?.max ? Math.min(100, Math.round(((p.cur || 0) * 100) / p.max)) : 0;
  return (
    <Modal title={mode === 'new' ? t('New Slide Show from Folder') : t('Add Images from Folder')} onClose={busy ? undefined : onClose} closeOnEscape={!busy} width={480}
      buttons={<><button onClick={onClose} disabled={busy}>{t('Cancel')}</button><button className="primary" onClick={ok} disabled={!ready}>{busy ? t('Importing...') : t('Import')}</button></>}>
      <div className="form-grid">
        {needName && (
          <>
            <span>{t('Title:')}</span>
            <input type="text" value={name} disabled={busy} onChange={(e) => { setName(e.target.value); setTouched(true); }} onKeyDown={(e) => e.key === 'Enter' && ok()} />
          </>
        )}
        <div className="col" style={{ gap: 4, gridColumn: '1 / -1' }}>
          <label className="chk"><input type="checkbox" checked={options.recursive} disabled={busy} onChange={(e) => setOpt({ recursive: e.target.checked })} />{t('Include images in subfolders')}</label>
          <label className="chk" title={canLink ? undefined : t('Pictures from a browser are always copied.')}>
            <input type="checkbox" checked={options.copy || !canLink} disabled={busy || !canLink} onChange={(e) => setOpt({ copy: e.target.checked })} />{t('Copy images to local folder before import')}
          </label>
        </div>
        <span>{t('Folder:')}</span>
        <div className="row" style={{ gap: 6 }}>
          <input type="text" className="grow" readOnly value={folder ? folder.dir ?? folder.name : ''} placeholder={t('(none selected)')} title={folder?.dir} />
          <button onClick={browse} disabled={picking || busy} autoFocus>{t('Browse...')}</button>
        </div>
      </div>
      {busy && (
        <div className="col" style={{ gap: 4, marginTop: 12 }}>
          <div className="row" style={{ gap: 8 }}>
            <progress value={pct} max={100} style={{ flex: 1 }} />
            <span className="muted" style={{ minWidth: 70, textAlign: 'right' }}>{p?.max ? `${p.cur ?? 0} / ${p.max}` : t('Looking...')}</span>
          </div>
          <div className="muted" style={{ fontSize: 12, overflow: 'hidden', textOverflow: 'ellipsis', whiteSpace: 'nowrap', minHeight: 16 }}>{p?.msg || ''}</div>
        </div>
      )}
    </Modal>
  );
}
