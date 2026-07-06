import type {
  DockerContainer,
  DockerImage,
  DockerNetwork,
  DockerOverview,
  DockerVolume,
} from "@/lib/types";
import { api } from "./client";

// Read-only Docker inspect endpoints (admin only). No mutations exist by design.

export function getDockerOverview() {
  return api.get<DockerOverview>("/docker/overview");
}

export function listDockerContainers() {
  return api.get<DockerContainer[]>("/docker/containers");
}

export function listDockerImages() {
  return api.get<DockerImage[]>("/docker/images");
}

export function listDockerVolumes() {
  return api.get<DockerVolume[]>("/docker/volumes");
}

export function listDockerNetworks() {
  return api.get<DockerNetwork[]>("/docker/networks");
}
