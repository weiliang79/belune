import { api } from "./client";

export type Features = {
  buildkit_available: boolean;
};

export function getFeatures() {
  return api.get<Features>("/features");
}
