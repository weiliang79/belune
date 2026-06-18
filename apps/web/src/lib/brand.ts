/**
 * Server branding shown in the sidebar identity block.
 *
 * Single-tenant: one install = one server, so this is static branding, not an
 * org switcher. `version` is a constant for now — wire it to a build-time define
 * or a backend `/version` field when available.
 */
export const BRAND = {
  name: "homelab-paas",
  version: "v0.0.17-alpha",
} as const;
