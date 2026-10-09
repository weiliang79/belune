import type { SelfUpdateStatus } from "@/lib/api/maintenance";

/** What the card has observed about update attempts so far.
 *
 *  Two facts, both of which only exist to stop a toast firing when it should
 *  not: whether this page has seen an attempt that was not already failed, and
 *  which failure it has already announced. */
export interface AnnounceState {
  /** True once this page has observed an attempt in a non-failed state. A
   *  failure read on the very first poll happened before the page existed, so
   *  the panel already shows it and a toast would be nagging. */
  sawLiveAttempt: boolean;
  /** The failure already announced, as `target|reason`. The status query keeps
   *  returning the same failed attempt — on every poll, and again on every
   *  refetch — and the toast is raised with a fixed id, so without this the
   *  same toast is re-raised repeatedly and its duration never expires. */
  announced: string;
}

export const INITIAL_ANNOUNCE_STATE: AnnounceState = {
  sawLiveAttempt: false,
  announced: "",
};

/** Decides whether a newly read attempt should raise the failure toast.
 *
 *  Pure, so the sequence that matters — idle, running, failed, failed again,
 *  then a second update that fails the same way — can be asserted directly.
 *  The component holds the returned state and does nothing else. */
export function nextAnnounce(
  state: AnnounceState,
  attempt: SelfUpdateStatus | undefined,
): { state: AnnounceState; announce: boolean } {
  if (!attempt?.state) return { state, announce: false };

  if (attempt.state !== "failed") {
    // Any non-failed reading proves this page is watching a live attempt, and
    // clears the announced failure so a SECOND update that fails identically
    // is still announced.
    return {
      state: { sawLiveAttempt: true, announced: "" },
      announce: false,
    };
  }

  if (!state.sawLiveAttempt) return { state, announce: false };

  const signature = `${attempt.target}|${attempt.reason ?? ""}`;
  if (state.announced === signature) return { state, announce: false };

  return {
    state: { ...state, announced: signature },
    announce: true,
  };
}
