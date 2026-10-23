import React, { useMemo } from 'react';

/** Renders text with regexp matches wrapped in <mark>. Pattern may carry a (?i) prefix. */
export function Highlight({ text, pattern }: { text: string; pattern: string }) {
  const parts = useMemo(() => {
    if (!pattern) return [text];
    let p = pattern;
    let flags = 'gu';
    if (p.startsWith('(?i)')) {
      p = p.slice(4);
      flags += 'i';
    }
    // JS has no \W* difference; convert go specific tokens
    try {
      const re = new RegExp(p, flags);
      const out: (string | React.ReactElement)[] = [];
      let last = 0;
      let m: RegExpExecArray | null;
      let guard = 0;
      while ((m = re.exec(text)) && guard < 500) {
        guard++;
        if (m[0] === '') {
          re.lastIndex++;
          continue;
        }
        out.push(text.slice(last, m.index));
        out.push(<mark key={m.index}>{m[0]}</mark>);
        last = m.index + m[0].length;
      }
      out.push(text.slice(last));
      return out;
    } catch {
      return [text];
    }
  }, [text, pattern]);
  return <>{parts}</>;
}
