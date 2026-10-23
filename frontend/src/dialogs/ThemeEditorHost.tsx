import { useState } from 'react';
import { useStore } from '../lib/store';
import { api } from '../lib/api';
import type { Settings } from '../lib/types';
import ThemeEditor from './ThemeEditor';

/**
 * The theme editor opened from the Theme menu, outside the Settings dialog. The settings it touches (the slide
 * transition, the theme in use) are saved as they change.
 */
export default function ThemeEditorHost({ onClose }: { onClose: () => void }) {
  const store = useStore();
  const [s, setSState] = useState<Settings>(() => JSON.parse(JSON.stringify(store.settings)));

  const save = async (next: Settings) => {
    setSState(next);
    try {
      await api.saveSettings(next);
      await store.reloadSettings();
    } catch { /* the editor keeps showing what was chosen */ }
  };
  const setS = (f: (x: Settings) => Settings) => { void save(f(s)); };

  const g = s.general;

  return (
    <ThemeEditor s={s} setS={setS} currentId={g.currentThemeId}
      onUse={(id) => { void save({ ...s, general: { ...s.general, currentThemeId: id } }); }}
      onClose={() => { onClose(); store.reloadSettings(); }} />
  );
}
