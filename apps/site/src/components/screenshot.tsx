/**
 * Theme-aware product screenshots. We ship a matched light/dark pair for every
 * shot under `public/screenshots/{light,dark}/<name>.png`; Fumadocs toggles the
 * theme with a `.dark` class on <html>, so we render both and let CSS pick.
 */
interface ScreenshotImgProps {
  /** Basename shared by the light and dark files, e.g. "app-overview". */
  name: string;
  /** Accessible description of what the screenshot shows. */
  alt: string;
}

/**
 * The bare light/dark <img> pair, no frame — for callers that supply their own
 * chrome (e.g. the landing page's `AssetFrame`).
 *
 * Plain <img>, not next/image: the site is a static export (`serve out`), where
 * next/image's default optimizer isn't available, and we intentionally render
 * both theme variants so CSS — not JS — picks one.
 */
export function ScreenshotImg({ name, alt }: ScreenshotImgProps) {
  return (
    <>
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={`/screenshots/light/${name}.png`}
        alt={alt}
        loading="lazy"
        className="block w-full dark:hidden"
      />
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={`/screenshots/dark/${name}.png`}
        alt={alt}
        loading="lazy"
        className="hidden w-full dark:block"
      />
    </>
  );
}

interface ScreenshotProps extends ScreenshotImgProps {
  /** Optional caption rendered beneath the frame. */
  caption?: string;
}

/** Framed, captioned screenshot used inline in docs MDX. */
export function Screenshot({ name, alt, caption }: ScreenshotProps) {
  return (
    <figure className="my-6">
      <div className="border-fd-border overflow-hidden rounded-lg border shadow-sm">
        <ScreenshotImg name={name} alt={alt} />
      </div>
      {caption && (
        <figcaption className="text-fd-muted-foreground mt-2 text-center text-sm">
          {caption}
        </figcaption>
      )}
    </figure>
  );
}
