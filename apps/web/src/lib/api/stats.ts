import type { Stats } from "@/lib/types";
import { api } from "./client";

export function getStats() {
  return api.get<Stats>("/stats");
}
