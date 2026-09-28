import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/client";
import { isNotFoundError } from "./query-error";

describe("isNotFoundError", () => {
  it("is true only for a 404", () => {
    expect(isNotFoundError(new ApiError("project not found", 404))).toBe(true);
  });

  it("is false for the failures that retrying actually fixes", () => {
    // The regression this guards: these used to render as "not found",
    // telling the reader their data was gone because the API restarted.
    expect(isNotFoundError(new ApiError("bad gateway", 502))).toBe(false);
    expect(isNotFoundError(new ApiError("internal error", 500))).toBe(false);
    expect(isNotFoundError(new ApiError("service unavailable", 503))).toBe(
      false,
    );
    expect(isNotFoundError(new ApiError("request timed out", 504))).toBe(false);
  });

  it("is false for an authorization failure, which is not absence either", () => {
    expect(isNotFoundError(new ApiError("unauthorized", 401))).toBe(false);
    expect(isNotFoundError(new ApiError("forbidden", 403))).toBe(false);
  });

  it("is false for a non-ApiError, including one that merely looks like one", () => {
    // A network failure surfaces as a plain TypeError from fetch, carrying no
    // status at all — it must not be mistaken for a 404.
    expect(isNotFoundError(new TypeError("Failed to fetch"))).toBe(false);
    expect(isNotFoundError(new Error("boom"))).toBe(false);
    expect(isNotFoundError({ status: 404 })).toBe(false);
    expect(isNotFoundError(undefined)).toBe(false);
    expect(isNotFoundError(null)).toBe(false);
  });
});
