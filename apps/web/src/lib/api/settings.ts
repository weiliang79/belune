import type { SettingEntry } from "@/lib/types";
import { api } from "./client";

export function getSettings() {
  return api.get<SettingEntry[]>("/settings");
}

export function updateSettings(settings: SettingEntry[]) {
  return api.put<{ status: string }>("/settings", settings);
}
