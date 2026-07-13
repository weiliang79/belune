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
    // "/app" + "/" would give "/app/", which is not what the app is asked for on
    // its own root; "/app" + "/users" gives "/app/users", which is.
    p = p === "/" ? internalPath : internalPath + p;
  }
  return p || "/";
}

/** A representative request for the preview: the app's own root, one level down. */
export function samplePublicRequest(publicPath: string): string {
  return publicPath === "/" ? "/users" : `${publicPath}/users`;
}
