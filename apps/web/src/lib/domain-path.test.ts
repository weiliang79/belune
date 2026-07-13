import { describe, expect, it } from "vitest";

import fixture from "./path-forwarding.fixture.json";
import {
  forwardedPath,
  normalizeInternalPath,
  normalizePublicPath,
  samplePublicRequest,
} from "./domain-path";

// The preview must agree with the Go route builder about what the container
// receives. It is the same fixture on both sides; see
// internal/proxy/caddy/routes_forwarding_test.go for the other half.
describe("forwardedPath (shared fixture)", () => {
  for (const c of fixture.cases) {
    it(c.name, () => {
      expect(
        forwardedPath(c.request, c.path, c.strip_path, c.internal_path),
      ).toBe(c.expected);
    });
  }
});

describe("normalizePublicPath", () => {
  it("treats a blank field as the whole host", () => {
    expect(normalizePublicPath("")).toBe("/");
    expect(normalizePublicPath("   ")).toBe("/");
  });

  it("roots a bare prefix and drops a trailing slash", () => {
    expect(normalizePublicPath("api")).toBe("/api");
    expect(normalizePublicPath("/api/")).toBe("/api");
  });
});

describe("normalizeInternalPath", () => {
  // Empty is a legitimate value here — it means "prepend nothing". Coercing it
  // to "/" the way the public path is coerced would prepend a bare slash and
  // rewrite /users into //users.
  it("keeps empty empty, and treats a lone slash as empty", () => {
    expect(normalizeInternalPath("")).toBe("");
    expect(normalizeInternalPath("/")).toBe("");
  });

  it("roots a bare prefix and drops a trailing slash", () => {
    expect(normalizeInternalPath("gf")).toBe("/gf");
    expect(normalizeInternalPath("/gf/")).toBe("/gf");
  });
});

describe("samplePublicRequest", () => {
  it("shows a request one level below the app's root", () => {
    expect(samplePublicRequest("/")).toBe("/users");
    expect(samplePublicRequest("/api")).toBe("/api/users");
  });
});
