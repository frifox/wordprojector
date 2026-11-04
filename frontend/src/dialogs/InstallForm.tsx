import { api } from '../lib/api';
import { pickFolder } from '../lib/files';
import { useStore } from '../lib/store';
import { t } from '../i18n';
import type { InstallInfo, UninstallOptions } from '../lib/types';

/** The choices of the install form. */
export interface InstallChoice {
  install: boolean;
  /** "do not ask again on startup" */
  skipAsk: boolean;
  dir: string;
  desktop: boolean;
  start: boolean;
}

export function initialChoice(info: InstallInfo, skipAsk: boolean): InstallChoice {
  return { install: false, skipAsk, dir: info.defaultDir, desktop: true, start: info.os === 'windows' };
}

/** The form of installing: keep the program as it is, or put it in the folder where programs belong. */
export function InstallForm({ info, value, onChange, showAsk = true }: { info: InstallInfo; value: InstallChoice; onChange: (v: InstallChoice) => void; showAsk?: boolean }) {
  const { boot } = useStore();
  const set = (p: Partial<InstallChoice>) => onChange({ ...value, ...p });
  const browse = async () => {
    const f = await pickFolder(boot.desktop, t('Select the Installation Folder'));
    if (f?.dir) set({ dir: f.dir });
  };
  const startLabel = info.os === 'windows' ? t('Create Start shortcut') : t('Create applications menu entry');
  const group: React.CSSProperties = { margin: '4px 0 0 22px', display: 'flex', flexDirection: 'column', gap: 6, alignItems: 'flex-start' };
  return (
    <div className="col" style={{ gap: 12 }}>
      <div>
        <label className="chk"><input type="radio" name="install-choice" checked={!value.install} onChange={() => set({ install: false })} />{t('Do not install - keep the app where it is')}</label>
        <div style={{ ...group, opacity: value.install ? 0.5 : 1 }}>
          {showAsk && (
            <label className="chk"><input type="checkbox" disabled={value.install} checked={value.skipAsk} onChange={(e) => set({ skipAsk: e.target.checked })} />{t('Do not ask on startup')}</label>
          )}
        </div>
      </div>
      <div>
        <label className="chk"><input type="radio" name="install-choice" checked={value.install} onChange={() => set({ install: true })} />{t('Install')}</label>
        <div style={{ ...group, opacity: value.install ? 1 : 0.5, alignItems: 'stretch' }}>
          <div className="row">
            <span>{t('Folder:')}</span>
            <input type="text" className="grow" style={{ minWidth: 0 }} disabled={!value.install} value={value.dir} onChange={(e) => set({ dir: e.target.value })} spellCheck={false} />
            <button disabled={!value.install} onClick={browse}>{t('Browse...')}</button>
          </div>
          <label className="chk"><input type="checkbox" disabled={!value.install} checked={value.desktop} onChange={(e) => set({ desktop: e.target.checked })} />{t('Create Desktop shortcut')}</label>
          {info.hasStart && (
            <label className="chk"><input type="checkbox" disabled={!value.install} checked={value.start} onChange={(e) => set({ start: e.target.checked })} />{startLabel}</label>
          )}
        </div>
      </div>
    </div>
  );
}

/** Installs with the choices; the program starts again from its new place, so the page goes away after this. */
export function runInstall(c: InstallChoice) {
  return api.install({ dir: c.dir, desktop: c.desktop, start: c.start });
}

/** The form of uninstalling: what to take away. */
export function UninstallForm({ info, value, onChange }: { info: InstallInfo; value: UninstallOptions; onChange: (v: UninstallOptions) => void }) {
  const set = (p: Partial<UninstallOptions>) => onChange({ ...value, ...p });
  const shortcuts = [info.desktop, info.start].filter(Boolean) as string[];
  const row = (checked: boolean, on: (b: boolean) => void, label: string, path: string | string[], disabled = false) => (
    <div>
      <label className="chk"><input type="checkbox" disabled={disabled} checked={checked && !disabled} onChange={(e) => on(e.target.checked)} />{label}</label>
      <div className="muted" style={{ margin: '1px 0 0 22px', wordBreak: 'break-all' }}>{[path].flat().map((p, i) => <div key={i}>{p}</div>)}</div>
    </div>
  );
  return (
    <div className="col" style={{ gap: 10 }}>
      {row(value.app, (b) => set({ app: b }), t('App folder'), info.appPath || '')}
      {row(value.shortcuts, (b) => set({ shortcuts: b }), t('Shortcuts'), shortcuts.length ? shortcuts : t('No shortcuts were made.'), !shortcuts.length)}
      {row(value.data, (b) => set({ data: b }), t('App data folder'), info.dataDir)}
      {row(value.docs, (b) => set({ docs: b }), t('App documents folder'), info.docsDir)}
    </div>
  );
}

export const defaultUninstall: UninstallOptions = { app: true, shortcuts: true, data: false, docs: false };
