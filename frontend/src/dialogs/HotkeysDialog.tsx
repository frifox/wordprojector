import Modal from '../components/Modal';
import { t } from '../i18n';
import { comboLabel, hotkeyMap } from '../lib/hotkeys';
import { useStore } from '../lib/store';

/** Help > Hotkeys: the keys that work anywhere in the program, with the ones the user has chosen in Settings. */
export default function HotkeysDialog({ onClose }: { onClose: () => void }) {
  const { settings } = useStore();
  const keys = hotkeyMap(settings.app.hotkeys);
  const groups: { title: string; rows: [string, string][] }[] = [
    { title: t('Tabs'), rows: [[t('Bible'), keys.tabBible], [t('Songs'), keys.tabSongs], [t('Slides'), keys.tabSlides]] },
    {
      title: t('Presentation'),
      rows: [
        [t('Go Live with the selection'), keys.play],
        [t('Preview the selection (hold)'), keys.preview],
        [t('Pause the running presentation'), keys.pause],
        [t('Stop the paused presentation'), keys.pause],
        [t('Stop and clear the running presentation'), keys.stop],
      ],
    },
    { title: t('App'), rows: [[t('Undo removing an item'), keys.undo], [t('Settings'), keys.settings], [t('Exit'), keys.exit]] },
  ];
  return (
    <Modal title={t('Hotkeys')} onClose={onClose} width={460} buttons={<button className="primary" onClick={onClose}>{t('Close')}</button>}>
      {groups.map((g, gi) => (
        <div key={gi} style={{ marginTop: gi ? 12 : 0 }}>
          <div style={{ fontWeight: 600, borderBottom: '1px solid var(--border)', paddingBottom: 3 }}>{g.title}</div>
          <table style={{ width: '100%', borderCollapse: 'collapse' }}>
            <tbody>
              {g.rows.map(([label, combo], i) => (
                <tr key={i}>
                  <td style={{ padding: '4px 8px 4px 0', width: 56, whiteSpace: 'nowrap' }}>{combo ? <span className="kbd">{comboLabel(combo)}</span> : <span className="muted">—</span>}</td>
                  <td style={{ padding: '4px 0' }}>{label}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      ))}
      <div className="muted" style={{ marginTop: 10 }}>{t('Some of these keys can be changed in Settings > Hotkeys.')}</div>
    </Modal>
  );
}
