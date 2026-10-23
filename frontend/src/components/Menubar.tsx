import React, { useEffect, useRef, useState } from 'react';

export interface MenuEntry {
  label?: string;
  shortcut?: string;
  icon?: string;
  checked?: boolean;
  disabled?: boolean;
  separator?: boolean;
  onClick?: () => void;
  submenu?: MenuEntry[];
}

export interface MenuDef {
  title: string;
  items: MenuEntry[];
  /** a small mark next to the title (something in the menu wants attention) */
  badge?: string;
}

export default function Menubar({ menus }: { menus: MenuDef[] }) {
  const [open, setOpen] = useState<number | null>(null);
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    if (open === null) return;
    const h = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(null);
    };
    const k = (e: KeyboardEvent) => {
      if (e.key === 'Escape') setOpen(null);
    };
    window.addEventListener('mousedown', h);
    window.addEventListener('keydown', k);
    return () => {
      window.removeEventListener('mousedown', h);
      window.removeEventListener('keydown', k);
    };
  }, [open]);
  return (
    <div className="menubar" ref={ref}>
      {menus.map((m, i) => (
        <div key={m.title} style={{ position: 'relative' }}>
          <div
            className={`menu-title ${open === i ? 'open' : ''}`}
            onMouseDown={(e) => {
              e.preventDefault();
              const wasOpen = open === i;
              setOpen(wasOpen ? null : i);
              if (wasOpen) return;
              // press, drag onto an item and release: that item is chosen in one gesture
              const title = e.currentTarget;
              // WebKit (the macOS app) freezes :hover and mouseenter while a button is held, so the item under the
              // pointer is highlighted by hand and the hover events React listens for are sent by hand
              let last: Element | null = null;
              const move = (ev: MouseEvent) => {
                const el = document.elementFromPoint(ev.clientX, ev.clientY);
                if (el === last) return;
                document.querySelectorAll('.menu-popup .mi.hot').forEach((x) => x.classList.remove('hot'));
                const item = el?.closest('.menu-popup .mi');
                if (item && !item.classList.contains('disabled')) item.classList.add('hot');
                last?.dispatchEvent(new MouseEvent('mouseout', { bubbles: true, relatedTarget: el }));
                el?.dispatchEvent(new MouseEvent('mouseover', { bubbles: true, relatedTarget: last }));
                last = el;
              };
              window.addEventListener('mousemove', move, true);
              const up = (ev: MouseEvent) => {
                window.removeEventListener('mouseup', up, true);
                window.removeEventListener('mousemove', move, true);
                document.querySelectorAll('.menu-popup .mi.hot').forEach((x) => x.classList.remove('hot'));
                const el = ev.target as HTMLElement | null;
                if (!el) return;
                if (title.contains(el)) return; // a plain click: the menu stays open
                const item = el.closest('.mi') as HTMLElement | null;
                if (item && !item.classList.contains('disabled')) {
                  if (!item.querySelector(':scope > .sc')?.textContent?.includes('▸')) item.click();
                } else if (!el.closest('.menu-popup')) {
                  setOpen(null); // released away from the menu
                }
              };
              window.addEventListener('mouseup', up, true);
            }}
            onMouseEnter={() => {
              if (open !== null && open !== i) setOpen(i);
            }}
          >
            {m.title}
            {m.badge && (
              <svg className="menu-badge" viewBox="0 0 24 24" aria-label={m.badge} role="img"><title>{m.badge}</title><circle cx="12" cy="12" r="10" /><path d="M12 17v-10" /><path d="M8 11l4 -4l4 4" /></svg>
            )}
          </div>
          {open === i && <MenuPopup items={m.items} close={() => setOpen(null)} />}
        </div>
      ))}
    </div>
  );
}

export function MenuPopup({ items, close, style }: { items: MenuEntry[]; close: () => void; style?: React.CSSProperties }) {
  const [sub, setSub] = useState<number | null>(null);
  return (
    <div className="menu-popup" style={{ top: '100%', left: 0, ...style }}>
      {items.map((it, i) =>
        it.separator ? (
          <div key={i} className="msep" />
        ) : (
          <div
            key={i}
            className={`mi ${it.disabled ? 'disabled' : ''}`}
            onMouseEnter={() => setSub(it.submenu ? i : null)}
            onClick={(e) => {
              e.stopPropagation();
              if (it.disabled || it.submenu) return;
              close();
              it.onClick?.();
            }}
            style={{ position: 'relative' }}
          >
            <span className="check">{it.checked ? '✓' : ''}</span>
            {it.icon && <img className="icon" src={`/icons/${it.icon}`} alt="" />}
            <span>{it.label}</span>
            {it.submenu && <span className="sc">▸</span>}
            {it.submenu && sub === i && <MenuPopup items={it.submenu} close={close} style={{ top: -4, left: '100%' }} />}
          </div>
        ),
      )}
    </div>
  );
}
