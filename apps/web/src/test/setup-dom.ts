import { cleanup } from "@testing-library/react";
import { afterEach } from "vitest";

// Testing Library registers this itself only when vitest runs with `globals:
// true`. This project does not — tests import from "vitest" explicitly — so
// without this the DOM from one test is still mounted during the next, and a
// query that should match one element finds several.
afterEach(() => {
  cleanup();
});
