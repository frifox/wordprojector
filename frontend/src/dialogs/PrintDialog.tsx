import { useState } from 'react';
import Modal from '../components/Modal';
import { t } from '../i18n';
import { useFonts, DropSelect } from '../components/Pickers';

/** Print preview: pick font/size then print through the browser (PDF via the print dialog). */
export default function PrintDialog({ title, text, fontSize = 12, onClose }: { title: string; text: string; fontSize?: number; onClose: () => void }) {
  const fonts = useFonts();
  const [family, setFamily] = useState('Arial');
  const [size, setSize] = useState(fontSize);
  const [content, setContent] = useState(text);
  const [margin, setMargin] = useState(0.5);
  const doPrint = () => {
    const esc = content.replace(/&/g, '&amp;').replace(/</g, '&lt;');
    const html = `<!doctype html><html><head><title>${title}</title><style>@page{margin:${margin}in}body{font-family:"${family}",Arial,sans-serif;font-size:${size}pt;white-space:pre-wrap;margin:0}</style></head><body>${esc}</body></html>`;
    // A hidden iframe works both in browsers and inside the desktop webview (no popups needed).
    const frame = document.createElement('iframe');
    frame.style.position = 'fixed';
    frame.style.right = '0';
    frame.style.bottom = '0';
    frame.style.width = '0';
    frame.style.height = '0';
    frame.style.border = '0';
    document.body.appendChild(frame);
    const doc = frame.contentDocument || frame.contentWindow?.document;
    if (!doc) return;
    doc.open();
    doc.write(html);
    doc.close();
    setTimeout(() => {
      frame.contentWindow?.focus();
      frame.contentWindow?.print();
      setTimeout(() => frame.remove(), 60000);
    }, 250);
  };
  return (
    <Modal title={title} onClose={onClose} width={760} height="85vh"
      buttons={<>
        <span className="grow" />
        <button onClick={onClose}>{t('Close')}</button>
        <button className="primary" onClick={doPrint}><img className="icon" src="/icons/print.png" alt="" />{t('Print / Save as PDF...')}</button>
      </>}>
      <div className="row">
        <span>{t('Font:')}</span>
        <DropSelect value={family} onChange={(v) => setFamily(String(v))} style={{ maxWidth: 200 }} options={(fonts.includes(family) ? fonts : [family, ...fonts]).map((f) => ({ value: f, label: f }))} />
        <input type="number" min={6} max={72} value={size} style={{ width: 60 }} onChange={(e) => setSize(Number(e.target.value) || 12)} />
        <span>{t('Margins (in):')}</span>
        <input type="number" min={0} max={3} step={0.1} value={margin} style={{ width: 60 }} onChange={(e) => setMargin(Number(e.target.value) || 0)} />
      </div>
      <textarea className="grow" value={content} onChange={(e) => setContent(e.target.value)} style={{ fontFamily: family, fontSize: `${size}pt`, background: '#fff', color: '#000', minHeight: 300, padding: 12 }} />
    </Modal>
  );
}
