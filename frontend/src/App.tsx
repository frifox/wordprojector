import { useCallback, useEffect, useState } from 'react';
import { api } from './lib/api';
import type { Bootstrap } from './lib/types';
import { StoreProvider } from './lib/store';
import Projector from './projector/Projector';
import Operator from './Operator';
import ConnectionError from './components/ConnectionError';
import { t } from './i18n';

export default function App() {
  const m = location.pathname.match(/^\/projector\/(\d+)/);
  if (m) return <Projector disp={Number(m[1])} />;
  return <OperatorApp />;
}

function OperatorApp() {
  const [boot, setBoot] = useState<Bootstrap | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [retrying, setRetrying] = useState(true);
  const connect = useCallback(() => {
    setRetrying(true);
    api.bootstrap().then((b) => { setError(null); setBoot(b); }).catch((e) => setError(String(e))).finally(() => setRetrying(false));
  }, []);
  useEffect(connect, [connect]);
  if (!boot && error) return <ConnectionError error={error} retrying={retrying} onRetry={connect} />;
  if (!boot) return <div className="conn-page"><div className="muted">{t('Loading…')}</div></div>;
  return (
    <StoreProvider boot={boot}>
      <Operator />
    </StoreProvider>
  );
}
