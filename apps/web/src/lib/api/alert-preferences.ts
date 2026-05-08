import type { AlertPreferences } from "@/lib/types";
import { api } from "./client";

export function getAlertPreferences() {
  return api.get<AlertPreferences>("/account/alert-preferences");
}

export function updateAlertPreferences(prefs: AlertPreferences) {
  return api.put<AlertPreferences>("/account/alert-preferences", prefs);
}
