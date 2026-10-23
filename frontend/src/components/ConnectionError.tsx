import { useEffect, useState } from 'react';
import { t } from '../i18n';

const RETRY_SECONDS = 4;

/** Full page shown when the Word Projector server can't be reached. Retries on its own. */
export default function ConnectionError({ error, retrying, onRetry }: { error: string; retrying: boolean; onRetry: () => void }) {
  const [left, setLeft] = useState(RETRY_SECONDS);
  useEffect(() => {
    if (retrying) return;
    setLeft(RETRY_SECONDS);
    const id = window.setInterval(() => {
      setLeft((s) => {
        if (s <= 1) {
          window.clearInterval(id);
          onRetry();
          return RETRY_SECONDS;
        }
        return s - 1;
      });
    }, 1000);
    return () => window.clearInterval(id);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [retrying, error]);

  return (
    <div className="conn-page" role="alert">
      <div className="conn-card">
        <svg className="conn-icon" viewBox="0 0 24 24" aria-hidden="true">
          <path d="M12 18l.01 0" />
          <path d="M9.172 15.172a4 4 0 0 1 5.656 0" />
          <path d="M6.343 12.343a8 8 0 0 1 11.314 0" />
          <path d="M3.515 9.515c4.686 -4.687 12.284 -4.687 16.97 0" />
          <path d="M3 3l18 18" />
        </svg>
        <h1>{t("Can't reach the Word Projector server")}</h1>
        <p>{t('The app is running, but its server is not responding. It may still be starting, or it may have stopped.')}</p>
        <ul>
          <li>{t('Using the desktop app: close it completely and open it again.')}</li>
          <li>{t('Running from source: start the backend server, then retry.')}</li>
        </ul>
        <div className="conn-actions">
          <button className="primary" onClick={onRetry} disabled={retrying}>{retrying ? t('Connecting…') : t('Try again')}</button>
          <span className="muted">{retrying ? '' : t('Retrying in {n}s', { n: left })}</span>
        </div>
        <details>
          <summary>{t('Technical details')}</summary>
          <pre>{error.replace(/^(Api)?Error:\s*/, '')}</pre>
        </details>
      </div>
    </div>
  );
}
