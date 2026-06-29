import { api } from "./client";

export type Features = {
  buildkit_available: boolean;
  instance_name: string;
};

export function getFeatures() {
  return api.get<Features>("/features");
}
