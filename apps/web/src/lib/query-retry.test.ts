import { describe, expect, it } from "vitest";
import { ApiError } from "@/lib/api/client";
import { shouldRetryQuery } from "./query-retry";

describe("shouldRetryQuery", () => {
  it("never retries a 429, however early the attempt", () => {
    // The one that matters. A retry here spends a request from a bucket that
    // is already empty, so being rate limited costs double.
    const rateLimited = new ApiError("too many requests", 429, 60);
    expect(shouldRetryQuery(0, rateLimited)).toBe(false);
  });

  it("never retries any other 4xx either", () => {
    for (const status of [400, 401, 403, 404, 409, 422, 499]) {
      expect(
        shouldRetryQuery(0, new ApiError("nope", status)),
        `status ${status}`,
      ).toBe(false);
    }
  });

  it("retries a 5xx exactly once", () => {
    const server = new ApiError("boom", 503);
    expect(shouldRetryQuery(0, server)).toBe(true);
    expect(shouldRetryQuery(1, server)).toBe(false);
  });

  it("retries a network failure exactly once", () => {
    // Not an ApiError at all: fetch rejected before any response existed,
    // which is the case a restarting API produces and the one the single
    // retry was added for.
    const offline = new TypeError("Failed to fetch");
    expect(shouldRetryQuery(0, offline)).toBe(true);
    expect(shouldRetryQuery(1, offline)).toBe(false);
  });

  it("treats a non-Error rejection as retryable rather than throwing", () => {
    expect(shouldRetryQuery(0, "something odd")).toBe(true);
    expect(shouldRetryQuery(0, undefined)).toBe(true);
  });

  it("does not use the 3xx or 5xx boundary by accident", () => {
    // 399 and 500 both sit outside the 4xx range the rule is about; an
    // off-by-one on either bound would silently change which failures retry.
    expect(shouldRetryQuery(0, new ApiError("x", 399))).toBe(true);
    expect(shouldRetryQuery(0, new ApiError("x", 500))).toBe(true);
  });
});
