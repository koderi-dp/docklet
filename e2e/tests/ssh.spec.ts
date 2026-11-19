import { expect, test } from "@playwright/test";
import { rowOf, useConnection } from "../support/app";
import { LOG_CONTAINER, SSH_CONNECTION } from "../support/names";

test("connects to the engine over SSH with a key", async ({ page }) => {
  await page.goto("/#/connections");
  await page.getByRole("button", { name: "Add connection" }).click();
  const dialog = page.getByRole("dialog", { name: "Add connection" });
  await dialog.getByRole("radio", { name: /SSH/ }).click();
  await dialog.getByLabel("Name").fill(SSH_CONNECTION);
  await dialog.getByLabel("Host").fill("engine");
  await dialog.getByLabel("User").fill("root");
  // The key and the engine's host key are installed in the Docklet container by its entrypoint.
  await dialog.getByLabel("Identity file (optional)").fill("/root/.ssh/id_ed25519");
  await dialog.getByRole("button", { name: "Test & Save" }).click();
  await expect(page.getByText(`Added ${SSH_CONNECTION}`)).toBeVisible({ timeout: 30_000 });

  const item = page.getByRole("main").getByRole("listitem").filter({ hasText: SSH_CONNECTION });
  await expect(item).toContainText("ssh://");

  // The same engine, now reached through ssh: its containers are listed.
  await page.goto("/#/containers");
  await useConnection(page, SSH_CONNECTION);
  await expect(rowOf(page, LOG_CONTAINER)).toContainText("Running");
});
