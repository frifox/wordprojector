// Phonetic matching of Latin typing against Cyrillic names ("bitie" ~ "Бытие",
// "ioan" ~ "Иоанна"). Cyrillic is first romanised, then both sides go through
// the same normaliser so they land on the same rough phonetic key.

const cyrToLatin: Record<string, string> = {
  а: 'a', б: 'b', в: 'v', г: 'g', д: 'd', е: 'e', ё: 'yo', ж: 'zh', з: 'z', и: 'i', й: 'y', к: 'k', л: 'l', м: 'm',
  н: 'n', о: 'o', п: 'p', р: 'r', с: 's', т: 't', у: 'u', ф: 'f', х: 'kh', ц: 'ts', ч: 'ch', ш: 'sh', щ: 'shch',
  ъ: '', ы: 'y', ь: '', э: 'e', ю: 'yu', я: 'ya',
  // Ukrainian
  і: 'i', ї: 'yi', є: 'ye', ґ: 'g',
};

export const hasCyrillic = (s: string) => /[Ѐ-ӿ]/.test(s);
export const hasLatin = (s: string) => /[a-z]/i.test(s);

/** Reduces romanised text to a phonetic key. */
function normalise(latin: string): string {
  let t = latin.toLowerCase();
  t = t
    .replace(/shch|sch/g, 's')
    .replace(/zh/g, 'x')
    .replace(/kh/g, 'h')
    .replace(/ch/g, 'c')
    .replace(/sh/g, 's')
    .replace(/ts/g, 'c')
    .replace(/y[uaeo]/g, (m) => m[1].trim())
    .replace(/[jy](?=[aeiou])/g, '') // iotation: Deyaniya ~ Dejan
    .replace(/[jy]/g, 'i')
    .replace(/w/g, 'v')
    .replace(/q/g, 'k');
  t = t.replace(/[^a-z0-9 ]/g, '');
  return t.replace(/(.)\1+/g, '$1'); // collapse double letters (Ioanna ~ Ioana)
}

/** Phonetic key of a Cyrillic string. */
export function keyFromCyrillic(s: string): string {
  let out = '';
  for (const ch of s.toLowerCase()) out += ch in cyrToLatin ? cyrToLatin[ch] : /[a-z0-9 ]/.test(ch) ? ch : '';
  return normalise(out);
}

/** Phonetic key of Latin typing. */
export function keyFromLatin(s: string): string {
  return normalise(s);
}

/** True when Latin typed text phonetically matches (is contained in) the Cyrillic name. */
export function phoneticContains(cyrillicName: string, typed: string): boolean {
  if (!hasCyrillic(cyrillicName) || !hasLatin(typed)) return false;
  const k = keyFromLatin(typed).trim();
  return k !== '' && keyFromCyrillic(cyrillicName).includes(k);
}

export function phoneticStartsWith(cyrillicWord: string, typed: string): boolean {
  if (!hasCyrillic(cyrillicWord) || !hasLatin(typed)) return false;
  const k = keyFromLatin(typed).trim();
  return k !== '' && keyFromCyrillic(cyrillicWord).startsWith(k);
}

export function phoneticEquals(cyrillicName: string, typed: string): boolean {
  if (!hasCyrillic(cyrillicName) || !hasLatin(typed)) return false;
  const k = keyFromLatin(typed).trim();
  return k !== '' && keyFromCyrillic(cyrillicName).trim() === k;
}

/** Range [start, end) of the shortest piece of a Cyrillic string that phonetically equals the typed Latin text. */
export function phoneticRange(cyrillic: string, typed: string): [number, number] | null {
  if (!hasCyrillic(cyrillic) || !hasLatin(typed)) return null;
  const k = keyFromLatin(typed).trim();
  if (!k) return null;
  const chars = Array.from(cyrillic);
  let offsets: number[] = [];
  let pos = 0;
  for (const ch of chars) { offsets.push(pos); pos += ch.length; }
  offsets.push(pos);
  for (let i = 0; i < chars.length; i++) {
    if (!(chars[i].toLowerCase() in cyrToLatin)) continue;
    let acc = '';
    for (let j = i; j < chars.length && j < i + k.length * 4 + 4; j++) {
      acc += chars[j];
      const key = keyFromCyrillic(acc);
      if (key === k) return [offsets[i], offsets[j + 1]];
      if (key.length > k.length + 2) break;
    }
  }
  return null;
}
