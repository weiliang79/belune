/** Humanize a snake/dot-cased action into a sentence, e.g. "deploy_application" → "Deploy application". */
export function actionLabel(action: string) {
  const s = action.replace(/[._]/g, " ").trim();
  return s.charAt(0).toUpperCase() + s.slice(1);
}

/** Tailwind soft-badge classes by action semantics. */
export function actionClass(action: string) {
  const a = action.toLowerCase();
  if (/(fail|delete|error|revoke|remove)/.test(a))
    return "bg-status-error-soft text-status-error";
  if (/(create|add|deploy|success|start|invite)/.test(a))
    return "bg-status-ready-soft text-status-ready";
  if (/(update|settings|restart|change|stop|end)/.test(a))
    return "bg-status-building-soft text-status-building";
  return "bg-elev text-text-muted";
}
