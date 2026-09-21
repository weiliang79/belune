import { describe, expect, it } from "vitest";

import { compareVersions } from "./use-update-available";

// The frontend decides "update available"; the backend decides "already up to
// date" with golang.org/x/mod/semver. These are the same pairs, and the two
// MUST agree — a disagreement either offers an update the API refuses or hides
// one it would accept. The second is what happened on the first real
// self-update: rc1's card said "up to date" for rc2 because parseInt("8-rc2")
// is 8, and the update had to be triggered from the browser console.
//
// Expected values were produced by running semver.Compare in Go, not derived
// by hand.
const sign = (n: number) => (n > 0 ? 1 : n < 0 ? -1 : 0);

describe("compareVersions agrees with golang.org/x/mod/semver", () => {
  it.each([
    // the pair that failed live on 2026-09-21
    ["0.1.8-rc2", "v0.1.8-rc1", 1],
    ["v0.1.8-rc1", "0.1.8-rc2", -1],
    // a pre-release is LOWER than the same version without one
    ["0.1.8", "0.1.8-rc2", 1],
    ["0.1.8-rc2", "0.1.8", -1],
    // the ordinary stable path, unchanged
    ["0.1.9", "0.1.8", 1],
    ["0.1.7", "v0.1.8", -1],
    ["0.1.7", "v0.1.7", 0],
    ["1.0.0", "0.9.9", 1],
    // numeric identifiers compare numerically, and lower than alphanumeric
    ["0.2.0-rc.10", "0.2.0-rc.2", 1],
    ["0.2.0-1", "0.2.0-alpha", -1],
    // a prefix is lower than the longer list
    ["0.2.0-rc", "0.2.0-rc.1", -1],
    // build metadata never affects order
    ["0.1.8+build7", "0.1.8", 0],
  ])("%s vs %s → %d", (a, b, expected) => {
    expect(sign(compareVersions(a, b))).toBe(expected);
  });

  // ⚠️ Looks wrong, is correct: "rc10" and "rc2" are single alphanumeric
  // identifiers, so they compare lexically and "rc10" < "rc2". Go does exactly
  // this. Do NOT "fix" it here without changing the backend to match — use
  // "rc.10" when numeric ordering is wanted.
  it('keeps "rc10" < "rc2", matching Go, however wrong it looks', () => {
    expect(sign(compareVersions("0.2.0-rc10", "0.2.0-rc2"))).toBe(-1);
  });

  // Whatever "up to date" means, it must be symmetric.
  it("is antisymmetric", () => {
    for (const [a, b] of [
      ["0.1.8-rc2", "0.1.8-rc1"],
      ["0.1.8", "0.1.8-rc2"],
      ["0.1.9", "0.1.8"],
    ]) {
      expect(sign(compareVersions(a, b))).toBe(-sign(compareVersions(b, a)));
    }
  });
});
