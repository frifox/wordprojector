import Modal from '../components/Modal';
import { t } from '../i18n';

export default function AboutDialog({ onClose, version }: { onClose: () => void; version: string }) {
  return (
    <Modal title={t('About Word Projector')} onClose={onClose} width={520} buttons={<button className="primary" onClick={onClose}>{t('Close')}</button>}>
      <div className="row" style={{ gap: 14, alignItems: 'flex-start' }}>
        <img src="/icons/softprojector.png" alt="" width={96} height={96} />
        <div className="col" style={{ gap: 8 }}>
          <div style={{ fontSize: 20, fontWeight: 600 }}>Word Projector {version}</div>
          <div>{t('An open source media projection program for churches: Bible verses, songs, pictures, media and announcements.')}</div>
          <div className="muted">
            {t('Word Projector is a re-implementation of SoftProjector (Copyright © Vladislav Kobzar and contributors) with a Go backend and a React user interface. It keeps the SoftProjector database and module formats (.sps, .spb, .spt, .spsc).')}
          </div>
          <div className="muted">{t('This program is free software released under the GNU General Public License version 3.')}</div>
          <div className="muted">softprojector.org</div>
        </div>
      </div>
    </Modal>
  );
}
