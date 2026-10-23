import { t } from '../i18n';
import type { Screen } from '../lib/types';

const MAP_W = 560;
const MAP_H = 220;

/**
 * The connected screens drawn the way the operating system arranges them. Clicking a screen turns projection
 * onto it on or off. `slots` holds the screen index of Display 1, 2, 3 ... (-1 = unused).
 */
export default function ScreenMap({ screens, slots, onChange }: { screens: Screen[]; slots: number[]; onChange: (slots: number[]) => void }) {
  const left = Math.min(...screens.map((s) => s.x));
  const top = Math.min(...screens.map((s) => s.y));
  const right = Math.max(...screens.map((s) => s.x + s.width));
  const bottom = Math.max(...screens.map((s) => s.y + s.height));
  const scale = Math.min(MAP_W / (right - left), MAP_H / (bottom - top));
  // a slot whose screen is no longer connected counts as unused (it is dropped on the next change)
  const used = slots.filter((i) => i >= 0 && screens.some((sc) => sc.index === i));

  const toggle = (index: number) => {
    const at = used.indexOf(index);
    let next: number[];
    if (at >= 0) {
      if (used.length === 1) return; // something always has to be projected onto
      next = used.filter((i) => i !== index);
    } else {
      next = [...used, index];
    }
    onChange(next);
  };

  return (
    <div className="screen-map" style={{ width: Math.round((right - left) * scale), height: Math.round((bottom - top) * scale) }}>
      {screens.map((sc) => {
        const disp = used.indexOf(sc.index);
        const on = disp >= 0;
        return (
          <button
            key={sc.index}
            className={`screen-tile ${on ? 'on' : ''}`}
            style={{ left: Math.round((sc.x - left) * scale), top: Math.round((sc.y - top) * scale), width: Math.round(sc.width * scale) - 3, height: Math.round(sc.height * scale) - 3 }}
            aria-pressed={on}
            title={on ? t('Click to stop projecting onto this screen') : t('Click to project onto this screen')}
            onClick={() => toggle(sc.index)}
          >
            <span className="screen-num">{sc.index + 1}</span>
            <span className="screen-res">{sc.width} × {sc.height}{sc.isPrimary ? ` · ${t('main')}` : ''}</span>
            <span className="screen-state">{on ? t('Display {n}', { n: disp + 1 }) : t('Off')}</span>
          </button>
        );
      })}
    </div>
  );
}
