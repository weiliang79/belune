import { createContext, useContext } from "react";
import type { AppMetricPoint } from "@/lib/types";

export interface AppMetricsContextValue {
  data: AppMetricPoint[];
  connected: boolean;
}

export const AppMetricsContext = createContext<AppMetricsContextValue>({
  data: [],
  connected: false,
});

export function useAppMetricsContext() {
  return useContext(AppMetricsContext);
}
