import type { RenderSlide } from '../lib/types';
import SlideView from './SlideView';

/**
 * Every slide is laid out on a canvas that is REF_H pixels high, whatever the real screen is, and then scaled
 * uniformly to fit. Font sizes (points x 4 reference pixels) therefore take the same share of the screen height
 * on every monitor, in the operator preview and in the settings thumbnails. Screens with the same aspect ratio get
 * an identical picture; a different ratio only changes how wide the canvas is, so the text stays the same size.
 */
export const REF_H = 1080;

export default function ScaledSlide({ slide, width, height }: { slide: RenderSlide; width: number; height: number }) {
  if (width <= 0 || height <= 0) return null;
  const scale = height / REF_H;
  const refW = Math.round(width / scale);
  return (
    <div style={{ position: 'absolute', left: 0, top: 0, width, height, overflow: 'hidden' }}>
      {/* zoom, not transform: the text is drawn at its real size, so it looks the same while a fade (which makes the
          browser draw the layer on its own, then magnify that picture) is running and after it */}
      <div style={{ position: 'absolute', left: 0, top: 0, width: refW, height: REF_H, zoom: scale }}>
        <SlideView slide={slide} width={refW} height={REF_H} />
      </div>
    </div>
  );
}
