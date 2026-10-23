/** Asks the live list to take keyboard focus (called after something goes live). */
export const focusLive = () => window.dispatchEvent(new Event('wp-focus-live'));
