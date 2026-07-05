// Human-readable labels for the common cron presets exposed in the backup
// config forms. Falls back to the raw expression for custom schedules, and to
// "Manual only" for an empty schedule.
const SCHEDULE_LABELS: Record<string, string> = {
  "0 * * * *": "Every hour",
  "0 0 * * *": "Every day at midnight",
  "0 13 * * *": "Every day at 1:00 PM",
  "0 0 * * 0": "Every week (Sunday midnight)",
  "0 0 1 * *": "Every month (1st, midnight)",
};

export function humanizeSchedule(schedule: string): string {
  if (!schedule) return "Manual only";
  return SCHEDULE_LABELS[schedule] ?? schedule;
}
