import { describe, expect, it } from "vitest";

import { projectLayoutMode } from "./project-layout-mode";

describe("projectLayoutMode", () => {
  it("shows chrome on the project overview itself", () => {
    expect(projectLayoutMode("/projects/P", "/projects/P")).toBe("chrome");
  });

  it("shows a transitioning skeleton crossing in from overview/settings into an app or database", () => {
    expect(projectLayoutMode("/projects/P/applications/A", "/projects/P")).toBe(
      "transitioning",
    );
    expect(
      projectLayoutMode("/projects/P/databases/D", "/projects/P/settings"),
    ).toBe("transitioning");
  });

  it("defers to <Outlet/> switching tabs within one app's detail page", () => {
    expect(
      projectLayoutMode(
        "/projects/P/applications/A/env",
        "/projects/P/applications/A",
      ),
    ).toBe("outlet");
  });

  it("defers to <Outlet/> jumping straight from one app to another", () => {
    expect(
      projectLayoutMode(
        "/projects/P/applications/B",
        "/projects/P/applications/A",
      ),
    ).toBe("outlet");
  });

  it("defers to <Outlet/> jumping straight from one database to another", () => {
    expect(
      projectLayoutMode("/projects/P/databases/D2", "/projects/P/databases/D1"),
    ).toBe("outlet");
  });

  it("defers to <Outlet/> even across a different project's child-detail page", () => {
    expect(
      projectLayoutMode(
        "/projects/P2/applications/B",
        "/projects/P1/applications/A",
      ),
    ).toBe("outlet");
  });

  it("defers to <Outlet/> on a cold load directly at a child-detail URL (resolvedLocation not yet ahead of location)", () => {
    expect(
      projectLayoutMode(
        "/projects/P/applications/A",
        "/projects/P/applications/A",
      ),
    ).toBe("outlet");
  });

  it("shows chrome leaving a child-detail page back to the project", () => {
    expect(projectLayoutMode("/projects/P", "/projects/P/applications/A")).toBe(
      "chrome",
    );
  });
});
