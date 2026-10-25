/** Asks the live list to take keyboard focus (called after something goes live). */
export const focusLive = () => window.dispatchEvent(new Event('wp-focus-live'));

/** Tells the live list that the Bible tab put verses on the displays (how many): one verse ends multi-verse mode. */
export const bibleShown = (verses: number) => window.dispatchEvent(new CustomEvent('wp-bible-shown', { detail: verses }));
