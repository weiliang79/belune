import { api } from "./client";

export type VersionInfo = {
  version: string;
  /** An update is in progress right now (not whether the last one worked —
   *  that is the admin-only update status). */
  updating: boolean;
  /** Whole seconds since it began; 0 when not updating. */
  updating_for: number;
};

export function getVersion() {
  return api.get<VersionInfo>("/version");
}
