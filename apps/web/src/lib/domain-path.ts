/**
 * The path arithmetic a request goes through between the browser and the app.
 *
 * Three fields decide it and none of them is meaningful alone, which is exactly
 * why the form shows the composed result: an operator can reason about "strip
 * /api" or "prepend /app" one at a time, but not about what a request actually
 * looks like by the time it arrives. Dokploy ships the same three fields and
 * makes you work it out in your head.
 */

/** Normalise a public path the way the server will: rooted, no trailing slash. */
export function normalizePublicPath(raw: string): string {
  const p = raw.trim();
  if (!p) return "/";
  const rooted = p.startsWith("/") ? p : `/${p}`;
  return rooted === "/" ? "/" : rooted.replace(/\/+$/, "");
}

/** Normalise an internal path. Empty is legitimate here — it means prepend nothing. */
export function normalizeInternalPath(raw: string): string {
  const p = raw.trim();
  if (!p || p === "/") return "";
  const rooted = p.startsWith("/") ? p : `/${p}`;
  return rooted.replace(/\/+$/, "");
}

/**
 * What the container receives, given what the browser asked for.
 *
 * Strip first, then prepend — the same order Caddy applies the two rewrites in,
 * and the only order that makes sense: take off what the public URL carries,
 * then add what the app demands.
 */
export function forwardedPath(
  requestPath: string,
  publicPath: string,
  stripPath: boolean,
  internalPath: string,
): string {
  let p = requestPath;

  if (stripPath && publicPath !== "/" && p.startsWith(publicPath)) {
    p = p.slice(publicPath.length) || "/";
  }
  if (internalPath) {
    // Plain concatenation, with no special case for the root. Caddy's rewrite is
    // literally `<internal>{http.request.uri}`, so a request that stripped down
    // to "/" becomes "/gf/" — with the trailing slash — and not "/gf". An earlier
    // version of this function "tidied" that away and the preview then disagreed
    // with the proxy about what the app receives, which is the one thing a
    // preview must never do.
    p = internalPath + p;
  }
  return p || "/";
}

/** A representative request for the preview: the app's own root, one level down. */
export function samplePublicRequest(publicPath: string): string {
  return publicPath === "/" ? "/users" : `${publicPath}/users`;
}
