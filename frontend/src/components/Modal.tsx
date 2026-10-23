import React, { useCallback, useEffect, useRef } from 'react';
import { t } from '../i18n';
import { filePickerOpen } from '../lib/picker';

interface Props {
  top?: boolean;
  title: string;
  onClose?: () => void;
  width?: number | string;
  height?: number | string;
  children: React.ReactNode;
  buttons?: React.ReactNode;
  closeOnEscape?: boolean;
  /** fills the whole window */
  full?: boolean;
}

// Open dialogs, topmost last: only the topmost one reacts to Esc.
const modalStack: object[] = [];

/**
 * Wraps a dialog's close action: when there are unsaved changes the user is asked to confirm first.
 * Use the returned function for Esc, clicking outside, and the Cancel button; call the real onClose after saving.
 */
export function useDiscardGuard(dirty: boolean, onClose: () => void, save?: () => void | Promise<void>) {
  const latest = useRef({ dirty, onClose, save });
  latest.current = { dirty, onClose, save };
  return useCallback(async () => {
    if (!latest.current.dirty) {
      latest.current.onClose();
      return;
    }
    const r = await confirmDialog({
      title: t('Discard changes?'),
      message: t('You have unsaved changes.'),
      // listed confirming choice first; the dialog shows them in the opposite order: Go Back, Discard, Save
      buttons: [
        ...(latest.current.save ? [{ label: t('Save'), value: 'save', primary: true }] : []),
        { label: t('Discard'), value: 'discard', primary: !latest.current.save },
        { label: t('Go Back'), value: 'back' },
      ],
    });
    if (r === 'discard') latest.current.onClose();
    else if (r === 'save') await latest.current.save?.(); // the dialog's own save closes it when it succeeds
  }, []);
}

export default function Modal({ title, onClose, width, height, children, buttons, closeOnEscape = true, top, full }: Props) {
  const pressedOutside = useRef(false);
  const token = useRef({});
  useEffect(() => {
    const me = token.current;
    modalStack.push(me);
    return () => { modalStack.splice(modalStack.indexOf(me), 1); };
  }, []);
  useEffect(() => {
    const h = (e: KeyboardEvent) => {
      // Esc that closes a file picker is not meant for the dialog behind it
      if (filePickerOpen()) return;
      // a field that uses Esc itself (cancelling its own edit) keeps it
      if ((e.target as HTMLElement | null)?.closest?.('[data-esc-own]')) return;
      if (e.key === 'Escape' && closeOnEscape && onClose && modalStack[modalStack.length - 1] === token.current) {
        e.stopPropagation();
        e.preventDefault();
        onClose();
      }
    };
    window.addEventListener('keydown', h, true);
    return () => window.removeEventListener('keydown', h, true);
  }, [onClose, closeOnEscape]);
  return (
    <div
      className={`modal-backdrop ${top ? 'top' : ''}`}
      onMouseDown={(e) => { e.stopPropagation(); pressedOutside.current = e.target === e.currentTarget; }}
      // a click that starts and ends on the dimmed area outside the dialog dismisses it, like Esc
      onClick={(e) => { if (pressedOutside.current && e.target === e.currentTarget && closeOnEscape && onClose) onClose(); pressedOutside.current = false; }}
    >
      <div className={`modal ${full ? 'full' : ''}`} style={full ? undefined : { width, height }} role="dialog" aria-label={title}>
        <div className="modal-title">{title}</div>
        <div className="modal-body">{children}</div>
        {buttons && <div className="modal-buttons">{buttons}</div>}
      </div>
    </div>
  );
}

export function confirmDialog(opts: {
  title: string;
  message: string;
  info?: string;
  buttons: { label: string; value: string; primary?: boolean }[];
}): Promise<string> {
  return new Promise((resolve) => {
    const ev = new CustomEvent('wp-confirm', { detail: { ...opts, resolve } });
    window.dispatchEvent(ev);
  });
}

export function alertDialog(title: string, message: string, info?: string): Promise<void> {
  return confirmDialog({ title, message, info, buttons: [{ label: 'OK', value: 'ok', primary: true }] }).then(() => undefined);
}

export function promptDialog(opts: { title: string; message: string; options: string[]; initial?: number }): Promise<string | null> {
  return new Promise((resolve) => {
    const ev = new CustomEvent('wp-prompt', { detail: { ...opts, resolve } });
    window.dispatchEvent(ev);
  });
}
