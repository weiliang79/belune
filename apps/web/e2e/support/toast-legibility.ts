import type { Page } from "@playwright/test";

/** What the probe observed about every toast title the page raised. */
export interface ToastObservation {
  /** The highest EFFECTIVE opacity (multiplied up the ancestor chain) the
   *  title's text ever reached on screen. */
  maxOpacity: number;
  /** Samples in which this toast was on screen but not the front of the stack —
   *  which is sonner's definition of "its contents are hidden". */
  obscuredSamples: number;
  /** Samples in which it was on screen at all, so an assertion can tell
   *  "legible" apart from "never rendered". */
  samples: number;
}

/**
 * Samples the toast stack while the drill runs, and reports what the operator
 * could actually have read.
 *
 * ⚠️ This exists because the thing it measures is the one thing neither of the
 * cheaper checks can see.
 *
 * `expect(locator).toBeVisible()` does NOT look at opacity — Playwright calls an
 * element visible when it has a non-empty bounding box and is not
 * `visibility: hidden`. A toast at `opacity: 0` is in the document, has a box,
 * has the right text, and is reported visible. jsdom is worse still: no layout
 * engine and only a partial `getComputedStyle`, so a jsdom test asserting "the
 * toast is in the document" passes while a human sees nothing.
 *
 * That is not hypothetical. v0.1.16 shipped it: a second toast was created
 * after the progress toast, sonner makes the newest one the front and renders
 *   `[data-sonner-toast][data-expanded=false][data-front=false][data-styled=true] > * { opacity: 0 }`
 * so the operator who clicked Update was the one client that could not see the
 * update's clock. It was running the whole time, two pixels behind a toast that
 * duplicated it.
 *
 * Three details are load-bearing:
 *
 *  - Opacity is EFFECTIVE, multiplied up the ancestor chain. The sonner rule
 *    above puts `opacity: 0` on the toast's CHILD, not the toast, so reading the
 *    `[data-sonner-toast]` element alone reports 1 and the bug walks through.
 *
 *  - It counts how often a toast was on screen but NOT the front of the stack,
 *    which is the discrete form of the same fact. That matters because opacity
 *    alone is timing-dependent — if the second toast arrives a moment after the
 *    first, the first is briefly legible before being covered, and any
 *    "was it ever legible?" measure says yes. `data-front` is a state, not an
 *    animation, so it does not care when the second toast arrived.
 *
 *  - The front/back counts are gathered only while `data-mounted=true` and
 *    `data-removed=false`. A toast on its way out legitimately yields the front
 *    to its replacement, and counting that would fail a working stack. No
 *    settling delay is needed on top: `data-front` is a state rather than an
 *    animation, and the opacity figure is a maximum, so neither is distorted by
 *    being sampled mid-fade. That matters — a delay would also be a minimum
 *    lifetime, and this journey's clock can be gone in well under a second.
 *
 * ⚠️ Being behind for ONE sample is normal and must be tolerated: a dismissal
 * and the next toast's arrival can land in the same React commit, so for one
 * tick the outgoing toast is already at index 1 and not yet marked removed.
 * Measured on a passing run: exactly one 50ms sample, with the text still at
 * full opacity because sonner fades content over 400ms. Callers compare against
 * a duration (see TOAST_SAMPLE_MS), not against zero — the defect is the same
 * state held for a toast's whole life, which was 10 seconds and ~200 samples.
 */
/** How often the probe samples. Exported so a caller can turn a sample count
 *  into the number that means something — how long a toast was unreadable. */
export const TOAST_SAMPLE_MS = 50;

export async function installToastProbe(page: Page): Promise<void> {
  // addInitScript rather than evaluate: it re-arms on every navigation, so the
  // probe cannot be lost to a redirect between being installed and being read.
  await page.addInitScript((SAMPLE_MS: number) => {
    const seen = new Map<
      string,
      { maxOpacity: number; obscuredSamples: number; samples: number }
    >();
    (window as unknown as { __toastProbe: typeof seen }).__toastProbe = seen;

    const effectiveOpacity = (el: Element): number => {
      let opacity = 1;
      for (let n: Element | null = el; n; n = n.parentElement) {
        const value = parseFloat(getComputedStyle(n).opacity);
        if (!Number.isNaN(value)) opacity *= value;
      }
      return opacity;
    };

    // setInterval, not requestAnimationFrame: rAF is throttled or suspended in a
    // background tab, and a probe that silently stopped sampling would report a
    // legible toast as invisible — a false failure, which is worse than no test.
    setInterval(() => {
      for (const toast of document.querySelectorAll("[data-sonner-toast]")) {
        const title = toast.querySelector("[data-title]");
        if (!title) continue;

        const text = title.textContent ?? "";
        const entry = seen.get(text) ?? {
          maxOpacity: 0,
          obscuredSamples: 0,
          samples: 0,
        };
        entry.maxOpacity = Math.max(entry.maxOpacity, effectiveOpacity(title));

        const onScreen =
          toast.getAttribute("data-mounted") === "true" &&
          toast.getAttribute("data-removed") !== "true";
        if (onScreen) {
          entry.samples += 1;
          if (toast.getAttribute("data-front") !== "true") {
            entry.obscuredSamples += 1;
          }
        }
        seen.set(text, entry);
      }
    }, SAMPLE_MS);
  }, TOAST_SAMPLE_MS);
}

/** One row per distinct toast title the probe saw, in the order they first
 *  appeared. The progress toast's title carries a live clock
 *  ("Belune is updating · 3s"), so it contributes one row per second — and each
 *  of those seconds is its own chance to have been hidden. */
export async function toastReport(
  page: Page,
): Promise<(ToastObservation & { title: string })[]> {
  const entries = await page.evaluate(() => {
    const probe = (
      window as unknown as {
        __toastProbe?: Map<string, ToastObservation>;
      }
    ).__toastProbe;
    return probe ? [...probe.entries()] : [];
  });
  return entries.map(([title, o]) => ({ title, ...o }));
}

/** The highest effective opacity reached by any toast whose title matches. 0
 *  when no such toast was ever rendered — "never shown" and "shown but
 *  invisible" are deliberately the same number, because they are the same thing
 *  to the operator. */
export function maxOpacity(
  report: (ToastObservation & { title: string })[],
  title: RegExp,
): number {
  return report
    .filter((row) => title.test(row.title))
    .reduce((max, row) => Math.max(max, row.maxOpacity), 0);
}
