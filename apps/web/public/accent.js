// Apply the persisted accent before React mounts, to avoid a colour flash.
//
// This lives in its own file rather than inline in index.html because the API
// serves a strict Content-Security-Policy (script-src 'self'), which blocks
// inline scripts outright. Inline, this silently never ran in production — the
// flash it exists to prevent happened on every load, and the console carried a
// CSP violation. Relaxing the policy with 'unsafe-inline' would defeat the point
// of having one; a hash would have to be recomputed on every edit.
//
// Loaded synchronously (no defer/async) so it runs before first paint.
try {
  var a = JSON.parse(localStorage.getItem("belune-accent"));
  if (a && a.state && a.state.accent === "emerald") {
    document.documentElement.dataset.accent = "emerald";
  }
} catch (e) {}
