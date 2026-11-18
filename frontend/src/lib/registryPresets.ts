/** Registry choices of the sign-in dialog. Docker Hub's host is "docker.io"; the backend maps it to the CLI's canonical key. */
export interface RegistryPreset {
  id: string;
  label: string;
  /** Host sent to the backend; empty for the custom entry. */
  host: string;
  hint: string;
}

export const CUSTOM_PRESET_ID = "custom";

export const REGISTRY_PRESETS: RegistryPreset[] = [
  { id: "docker.io", label: "Docker Hub", host: "docker.io", hint: "Use a personal access token instead of your password (Docker Hub account settings, Personal access tokens)." },
  { id: "ghcr.io", label: "GitHub Container Registry (ghcr.io)", host: "ghcr.io", hint: "Your GitHub user name and a personal access token with read:packages (add write:packages to push)." },
  { id: "registry.gitlab.com", label: "GitLab registry (registry.gitlab.com)", host: "registry.gitlab.com", hint: "Your GitLab user name and a personal or deploy token with read_registry (write_registry to push)." },
  { id: "quay.io", label: "Quay (quay.io)", host: "quay.io", hint: "Your Quay user name and an application token or encrypted password; robot accounts use namespace+robot." },
  { id: CUSTOM_PRESET_ID, label: "Custom host", host: "", hint: "Host with optional port, for example registry.example.com:5000. Registries on 127.0.0.1 may use plain HTTP." },
];

/** The preset for a registry host; unknown hosts map to the custom entry. */
export function presetForHost(host: string): RegistryPreset {
  const h = host.trim().toLowerCase();
  return REGISTRY_PRESETS.find((p) => p.host && p.host === h) ?? REGISTRY_PRESETS[REGISTRY_PRESETS.length - 1];
}

/**
 * The registry an operation failed to authenticate with, read from the backend's
 * pull/push failure text ("No credentials for <host>. ..." or "<host> rejected the saved credentials ...").
 */
export function authFailureHost(error: string | undefined): string | null {
  if (!error) return null;
  const m = /No credentials for (\S+?)\. /.exec(error) ?? /(?:^|\s)(\S+) rejected the saved credentials/.exec(error);
  return m ? m[1] : null;
}
