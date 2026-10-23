import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import './index.css'
import App from './App.tsx'

// No browser context menu (Copy / Select All ...) except in fields the user types in, where it offers copy and paste.
// Everything else is left for the app's own right-click menus later.
const TEXT_INPUTS = new Set(['text', 'search', 'number', 'url', 'tel', 'email', 'password'])
window.addEventListener('contextmenu', (e) => {
  const el = e.target instanceof Element ? e.target : null
  const field = el?.closest('input, textarea, [contenteditable=""], [contenteditable="true"]')
  if (field instanceof HTMLTextAreaElement || (field instanceof HTMLInputElement && TEXT_INPUTS.has(field.type)) || (field instanceof HTMLElement && field.isContentEditable)) return
  e.preventDefault()
}, true)

// Shift + Up / Down in a number field changes it by 10 steps instead of 1
window.addEventListener('keydown', (e) => {
  const el = e.target;
  if (!e.shiftKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown') || !(el instanceof HTMLInputElement) || el.type !== 'number' || el.readOnly || el.disabled) return;
  e.preventDefault();
  const step = Number(el.step) > 0 ? Number(el.step) : 1;
  const dir = e.key === 'ArrowUp' ? 1 : -1;
  let v = (Number(el.value) || 0) + dir * step * 10;
  if (el.min !== '' && v < Number(el.min)) v = Number(el.min);
  if (el.max !== '' && v > Number(el.max)) v = Number(el.max);
  v = Math.round(v * 1e6) / 1e6;
  // the native setter, so React sees the change
  Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set?.call(el, String(v));
  el.dispatchEvent(new Event('input', { bubbles: true }));
}, true);

createRoot(document.getElementById('root')!).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
