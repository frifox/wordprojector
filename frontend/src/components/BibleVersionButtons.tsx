import { useMemo, useRef, useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import { t } from '../i18n';

/**
 * The Bible version buttons: which versions the display shows (up to three, in the order of the buttons; drag to
 * re-order). Used above the live area and above the theme editor's Bible preview.
 */
export default function BibleVersionButtons({ large = false }: { large?: boolean }) {
  const { settings, bibles, reloadSettings } = useStore();
  // Bible versions currently projected on display 1 (primary, secondary, third)
  const bv = settings.bible[0];
  const enabled = [bv.primary, bv.secondary, bv.trinary].filter((v) => v !== 'none');
  const saveVersions = async (next: string[]) => {
    if (next.length === 0 || next.length > 3) return;
    if (next.join(',') === enabled.join(',')) return;
    const s2 = JSON.parse(JSON.stringify(settings));
    s2.bible[0].primary = next[0] ?? 'none';
    s2.bible[0].secondary = next[1] ?? 'none';
    s2.bible[0].trinary = next[2] ?? 'none';
    await api.saveSettings(s2);
    await reloadSettings();
  };
  // Buttons keep their position when toggled; dragging rearranges the buttons, and the
  // translations on the display follow the button order.
  const [btnOrder, setBtnOrder] = useState<string[]>(() => settings.app.bibleButtonOrder ?? []);
  const orderedBibles = useMemo(() => {
    const ids = bibles.map((b) => String(b.id));
    const base = btnOrder.filter((id) => ids.includes(id));
    const known = new Set(base);
    // first run: lit versions first (in display order), then the rest
    const rest = [...enabled.filter((id) => ids.includes(id)), ...ids].filter((id) => !known.has(id));
    const seen = new Set<string>();
    const order = [...base, ...rest].filter((id) => (seen.has(id) ? false : (seen.add(id), true)));
    return order.map((id) => bibles.find((b) => String(b.id) === id)!);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [bibles, btnOrder, enabled.join(',')]);
  const orderIds = orderedBibles.map((b) => String(b.id));
  const persistOrder = (o: string[]) => {
    setBtnOrder(o);
    api.patchApp({ bibleButtonOrder: o }).then(() => reloadSettings()).catch(() => {});
  };
  const toggleVersion = (id: string) => {
    const lit = new Set(enabled.includes(id) ? enabled.filter((v) => v !== id) : [...enabled, id]);
    persistOrder(orderIds);
    saveVersions(orderIds.filter((v) => lit.has(v)));
  };
  const dragVer = useRef<string | null>(null);
  const [dragId, setDragId] = useState<string | null>(null);
  const [overVer, setOverVer] = useState<string | null>(null);
  const btnRefs = useRef<Record<string, HTMLButtonElement | null>>({});
  const widths = useRef<Record<string, number>>({});
  const slots = useRef<{ id: string; left: number; right: number }[]>([]);
  // Hit-test against the buttons' resting positions (taken when the drag starts). Using the
  // element under the pointer would flip-flop as the buttons slide under the cursor.
  const slotAt = (clientX: number): string | null => {
    const list = slots.current;
    if (!list.length) return null;
    if (clientX < list[0].left) return list[0].id;
    for (const sl of list) if (clientX >= sl.left && clientX < sl.right) return sl.id;
    return list[list.length - 1].id;
  };
  // order the buttons would have if dropped on `target` (moving right lands after it, left before it)
  const movedOrder = (from: string, target: string) => {
    const next = orderIds.filter((v) => v !== from);
    next.splice(orderIds.indexOf(target), 0, from);
    return next;
  };
  // while dragging, the other buttons slide aside to preview the drop position
  const slideOffset = (id: string): number => {
    if (!dragId || !overVer || dragId === overVer) return 0;
    const preview = movedOrder(dragId, overVer);
    const left = (order: string[]) => order.slice(0, order.indexOf(id)).reduce((sum, v) => sum + (widths.current[v] ?? 0) - 1, 0);
    return left(preview) - left(orderIds);
  };
  const dropVersion = (target: string) => {
    const from = dragVer.current;
    dragVer.current = null;
    setDragId(null);
    setOverVer(null);
    if (!from || from === target) return;
    const next = movedOrder(from, target);
    persistOrder(next);
    saveVersions(next.filter((v) => enabled.includes(v)));
  };
  if (bibles.length <= 1) return null;
  return (
    <span
      className="btn-group"
      onDragOver={(e) => {
        if (!dragVer.current) return;
        e.preventDefault();
        const target = slotAt(e.clientX);
        setOverVer((o) => (o === target ? o : target));
      }}
      onDrop={(e) => {
        e.preventDefault();
        dropVersion(slotAt(e.clientX) ?? overVer ?? '');
      }}
    >
      {orderedBibles.map((b) => {
        const id = String(b.id);
        const on = enabled.includes(id);
        const blocked = !on && enabled.length >= 3;
        const last = on && enabled.length === 1;
        return (
          <button
            key={b.id}
            ref={(el) => { btnRefs.current[id] = el; }}
            style={{ transform: dragId ? `translateX(${slideOffset(id)}px)` : undefined, opacity: dragId === id ? 0.45 : 1, transition: dragId ? 'transform 0.15s ease' : undefined }}
            className={`${large ? '' : 'small '}ver-btn ${on ? 'on' : ''}`}
            disabled={blocked || last}
            draggable={orderedBibles.length > 1}
            title={`${b.name} — ${t('drag to change the order')}`}
            onDragStart={(e) => {
              dragVer.current = id;
              orderIds.forEach((v) => { widths.current[v] = btnRefs.current[v]?.getBoundingClientRect().width ?? 0; });
              slots.current = orderIds.map((v) => {
                const r = btnRefs.current[v]!.getBoundingClientRect();
                return { id: v, left: r.left, right: r.right };
              });
              e.dataTransfer.effectAllowed = 'move';
              e.dataTransfer.setData('text/plain', id);
              // set after the drag image is captured
              window.setTimeout(() => setDragId(id), 0);
            }}

            onDragEnd={() => { dragVer.current = null; setDragId(null); setOverVer(null); }}
            onClick={() => toggleVersion(id)}
          >
            {b.abbr || b.name.slice(0, 4)}
          </button>
        );
      })}
    </span>
  );
}
