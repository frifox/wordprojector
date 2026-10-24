import { api, uploadFiles } from './api';
import { trackPicker, pickerActive } from './picker';
import { t } from '../i18n';

export interface FileFilter {
  name: string;
  pattern: string; // "*.sps;*.xml"
}

/**
 * Picks files using the native dialog in desktop mode, or a browser file
 * input otherwise (the files are uploaded and server paths returned).
 */
export function pickFiles(desktop: boolean, title: string, filters: FileFilter[], multiple: boolean): Promise<string[]> {
  if (pickerActive()) return Promise.resolve([]); // one dialog at a time
  return trackPicker(pickFilesNow(desktop, title, filters, multiple));
}

async function pickFilesNow(desktop: boolean, title: string, filters: FileFilter[], multiple: boolean): Promise<string[]> {
  if (desktop) {
    try {
      const r = await api.dialogOpen(title, filters, multiple);
      return r.cancelled ? [] : r.paths;
    } catch (e) {
      if ((e as { status?: number }).status !== 501) throw e;
    }
  }
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.multiple = multiple;
    const accept = filters
      .flatMap((f) => f.pattern.split(';'))
      .map((p) => p.trim().replace(/^\*/, ''))
      .filter((p) => p && p !== '.*')
      .join(',');
    if (accept) input.accept = accept;
    input.onchange = async () => {
      if (!input.files || input.files.length === 0) return resolve([]);
      try {
        resolve(await uploadFiles(input.files));
      } catch {
        resolve([]);
      }
    };
    input.style.display = 'none';
    document.body.appendChild(input);
    input.addEventListener('cancel', () => resolve([]));
    input.click();
    setTimeout(() => input.remove(), 60000);
  });
}

export interface FolderPick {
  name: string; // folder name
  dir?: string; // server side path (desktop)
  paths?: string[]; // uploaded images (browser)
  topPaths?: string[]; // those of them that are not in a subfolder
}

/** Picks a folder with the native dialog; in a browser the images of the chosen folder are uploaded. */
export function pickFolder(desktop: boolean, title: string): Promise<FolderPick | null> {
  if (pickerActive()) return Promise.resolve(null);
  return trackPicker(pickFolderNow(desktop, title));
}

async function pickFolderNow(desktop: boolean, title: string): Promise<FolderPick | null> {
  if (desktop) {
    try {
      const r = await api.dialogFolder(title);
      return r.cancelled || !r.path ? null : { name: r.name || r.path, dir: r.path };
    } catch (e) {
      if ((e as { status?: number }).status !== 501) throw e;
    }
  }
  return new Promise((resolve) => {
    const input = document.createElement('input');
    input.type = 'file';
    input.setAttribute('webkitdirectory', '');
    input.onchange = async () => {
      const files = Array.from(input.files ?? []).filter((f) => /\.(jpe?g|png|gif|bmp|webp|tiff?)$/i.test(f.name) && !f.name.startsWith('.'));
      const rel = (input.files?.[0] as (File & { webkitRelativePath?: string }) | undefined)?.webkitRelativePath ?? '';
      const name = rel.split('/')[0] || '';
      try {
        const paths = files.length ? await uploadFiles(files) : [];
        // uploadFiles answers in the order of the files: the ones straight in the folder have one slash in their path
        const top = paths.filter((_, i) => ((files[i] as File & { webkitRelativePath?: string }).webkitRelativePath ?? '').split('/').length <= 2);
        resolve({ name, paths, topPaths: top });
      } catch {
        resolve(null);
      }
    };
    input.style.display = 'none';
    document.body.appendChild(input);
    input.addEventListener('cancel', () => resolve(null));
    input.click();
    setTimeout(() => input.remove(), 60000);
  });
}

/** Triggers a browser download of a server side export (browser mode). */
export function downloadURL(url: string) {
  const a = document.createElement('a');
  a.href = url;
  a.download = '';
  document.body.appendChild(a);
  a.click();
  a.remove();
}

// the names are looked up when the dialog opens, in the language chosen then
export const imageFilter: FileFilter = { get name() { return t('Images'); }, pattern: '*.jpg;*.jpeg;*.png;*.gif;*.bmp;*.webp;*.tif;*.tiff' };
export const mediaFilter: FileFilter = {
  get name() { return t('Media Files'); },
  pattern: '*.mp3;*.aac;*.ogg;*.oga;*.wma;*.wav;*.m4a;*.flac;*.wmv;*.avi;*.mkv;*.flv;*.mp4;*.mpg;*.mpeg;*.mov;*.ogv;*.ts;*.webm;*.m4v',
};
