import { DropSelect } from './Pickers';
import { useEffect, useState } from 'react';
import Modal from './Modal';
import { t } from '../i18n';

interface Confirm {
  title: string;
  message: string;
  info?: string;
  buttons: { label: string; value: string; primary?: boolean }[];
  choices?: { label: string; value: string }[];
  resolve: (v: string) => void;
}
interface Prompt {
  title: string;
  message: string;
  options: string[];
  initial?: number;
  resolve: (v: string | null) => void;
}

/** Hosts the global confirm/prompt dialogs raised through confirmDialog/promptDialog. */
export default function DialogHost() {
  const [confirm, setConfirm] = useState<Confirm | null>(null);
  const [prompt, setPrompt] = useState<Prompt | null>(null);
  const [choice, setChoice] = useState(0);
  const [picked, setPicked] = useState(0); // the radio button chosen in a confirm dialog
  useEffect(() => {
    const c = (e: Event) => {
      setPicked(0);
      setConfirm((e as CustomEvent).detail as Confirm);
    };
    const p = (e: Event) => {
      const d = (e as CustomEvent).detail as Prompt;
      setChoice(d.initial || 0);
      setPrompt(d);
    };
    window.addEventListener('wp-confirm', c);
    window.addEventListener('wp-prompt', p);
    return () => {
      window.removeEventListener('wp-confirm', c);
      window.removeEventListener('wp-prompt', p);
    };
  }, []);
  return (
    <>
      {confirm && (
        <Modal
          top
          title={confirm.title}
          onClose={() => {
            const cancel = confirm.buttons.find((b) => b.value === 'cancel' || b.value === 'no');
            confirm.resolve(cancel ? cancel.value : confirm.buttons[confirm.buttons.length - 1].value);
            setConfirm(null);
          }}
          // the cancelling choice comes first and the confirming one last
          buttons={[...confirm.buttons].reverse().map((b) => (
            <button
              key={b.value}
              className={b.primary ? 'primary' : ''}
              autoFocus={b.primary}
              onClick={() => {
                confirm.resolve(b.primary && confirm.choices?.length ? confirm.choices[picked].value : b.value);
                setConfirm(null);
              }}
            >
              {b.label}
            </button>
          ))}
        >
          <div style={{ whiteSpace: 'pre-wrap', minWidth: 300 }}>{confirm.message}</div>
          {confirm.choices && (
            <div className="col" style={{ gap: 4, marginTop: 8 }}>
              {confirm.choices.map((c, i) => (
                <label key={c.value} className="chk"><input type="radio" name="confirm-choice" checked={picked === i} onChange={() => setPicked(i)} />{c.label}</label>
              ))}
            </div>
          )}
          {confirm.info && <div className="muted" style={{ whiteSpace: 'pre-wrap', marginTop: 8 }}>{confirm.info}</div>}
        </Modal>
      )}
      {prompt && (
        <Modal
          top
          title={prompt.title}
          onClose={() => {
            prompt.resolve(null);
            setPrompt(null);
          }}
          buttons={
            <>
              <button
                onClick={() => {
                  prompt.resolve(null);
                  setPrompt(null);
                }}
              >
                {t('Cancel')}
              </button>
              <button
                className="primary"
                onClick={() => {
                  prompt.resolve(prompt.options[choice] ?? null);
                  setPrompt(null);
                }}
              >
                {t('OK')}
              </button>
            </>
          }
        >
          <div style={{ whiteSpace: 'pre-wrap', minWidth: 320 }}>{prompt.message}</div>
          <DropSelect autoFocus value={choice} onChange={(v) => setChoice(Number(v))} options={prompt.options.map((o, i) => ({ value: i, label: o }))} />
        </Modal>
      )}
    </>
  );
}
