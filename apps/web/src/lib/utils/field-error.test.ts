import { describe, expect, it } from "vitest";

import { fieldError } from "./field-error";

describe("fieldError", () => {
  it("returns undefined for no errors", () => {
    expect(fieldError([])).toBeUndefined();
  });

  it("passes through a plain string error (custom function validators)", () => {
    expect(fieldError(["Name is required"])).toBe("Name is required");
  });

  it("unwraps a { message } error (Zod schema validators)", () => {
    expect(fieldError([{ message: "Invalid email" }])).toBe("Invalid email");
  });

  it("only looks at the first error", () => {
    expect(fieldError(["First", "Second"])).toBe("First");
    expect(fieldError([{ message: "First" }, { message: "Second" }])).toBe(
      "First",
    );
  });

  it("treats a falsy first error as no error", () => {
    expect(fieldError([undefined])).toBeUndefined();
    expect(fieldError([""])).toBeUndefined();
  });

  it("falls through to undefined for a message-less object", () => {
    expect(fieldError([{}])).toBeUndefined();
  });
});
