import { api } from "./client";

export type VersionInfo = {
  version: string;
};

export function getVersion() {
  return api.get<VersionInfo>("/version");
}
