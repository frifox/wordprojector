import { Highlight } from './Highlight';

// Same title words the backend recognises as the start of a stanza (see internal/songs/stanza.go).
const TITLE = /^&?(Verse|Куплет|Strophe|Verš|Chorus|Sbor|Refrain|Припев|Приспів|Refrén|Slide|Слайд|Dia|Snímek|Insert|Вставка|Einfügung|Vložka|Intro|Вступление|Einleitung|Úvod|Ending|Окончание|Закінчення|Ende|Závěr)/;

/** A song stanza whose title line ("Verse 1", "Chorus"…) is slightly dimmed. */
export default function StanzaText({ text, pattern }: { text: string; pattern?: string }) {
  const nl = text.indexOf('\n');
  const first = nl < 0 ? text : text.slice(0, nl);
  const body = nl < 0 ? '' : text.slice(nl);
  const hasTitle = TITLE.test(first);
  const render = (s: string) => (pattern ? <Highlight text={s} pattern={pattern} /> : s);
  if (!hasTitle) return <>{render(text)}</>;
  return (
    <>
      <span style={{ opacity: 0.55 }}>{first}</span>
      {render(body)}
    </>
  );
}
