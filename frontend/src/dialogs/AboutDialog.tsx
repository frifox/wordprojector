import Modal from '../components/Modal';
import { t } from '../i18n';
import { api } from '../lib/api';
import { useStore } from '../lib/store';

const PROJECT_URL = 'https://github.com/frifox/wordprojector';

export default function AboutDialog({ onClose, version }: { onClose: () => void; version: string }) {
  const { boot } = useStore();
  // the program's own window cannot browse: the desktop app asks the computer to open the page in its browser
  const openProject = (e: React.MouseEvent) => {
    if (!boot.desktop) return; // a browser follows the link itself
    e.preventDefault();
    api.openURL(PROJECT_URL).catch(() => {});
  };
  return (
    <Modal title={t('About Word Projector')} onClose={onClose} width={520} buttons={<button className="primary" onClick={onClose}>{t('Close')}</button>}>
      <div className="row" style={{ gap: 14, alignItems: 'flex-start' }}>
        <img src="/icons/softprojector.png" alt="" width={96} height={96} />
        <div className="col" style={{ gap: 8 }}>
          <div style={{ fontSize: 20, fontWeight: 600 }}>Word Projector {version}</div>
          <div>{t('An open source media projection program for churches:')}<br />{t('Simple app to display Bible, songs, slide shows.')}</div>
          <div className="col" style={{ gap: 4, alignItems: 'flex-start' }}>
            <div>{t('Need help, found a bug or have an idea?')}<br />{t('Ask on the project page on GitHub:')}</div>
            <a href={PROJECT_URL} target="_blank" rel="noreferrer" onClick={openProject}>{PROJECT_URL}</a>
          </div>
        </div>
      </div>
      <div className="muted" style={{ marginTop: 10 }}>
        {t('Word Projector is a re-implementation of SoftProjector (Copyright © Vladislav Kobzar and contributors) with a Go backend and a React user interface.')}
      </div>
      <div className="muted" style={{ marginTop: 6 }}>{t('This program is free software released under the GNU GPL v3.')}</div>
    </Modal>
  );
}
