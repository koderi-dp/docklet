/** Names the seed creates and the tests look for. */
export const TCP_CONNECTION = "E2E engine (TCP)";
export const SSH_CONNECTION = "E2E engine (SSH)";

/** Prints a line every second, for the log test. */
export const LOG_CONTAINER = "e2e-ticker";
/** Stopped and started by a test. */
export const TOGGLE_CONTAINER = "e2e-toggle";
/** Removed by a test. */
export const DOOMED_CONTAINER = "e2e-doomed";

export const BASE_IMAGE = "busybox:1.36";
/** Only in the test registry until the pull test fetches it. */
export const REGISTRY_IMAGE = "registry:5000/e2e/busybox:smoke";

export const COMPOSE_PROJECT = "e2e-demo";
export const COMPOSE_DIR = "/projects/demo";
