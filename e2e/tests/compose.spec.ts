import { expect, test } from "@playwright/test";
import { openPage, rowOf } from "../support/app";
import { containerState, dockletSh, engineDocker } from "../support/engine";
import { COMPOSE_PROJECT, TCP_CONNECTION } from "../support/names";

test("brings a Compose project up and down", async ({ page }) => {
  await openPage(page, "/compose", TCP_CONNECTION);
  const row = rowOf(page, COMPOSE_PROJECT);
  await expect(row).toContainText("Discovered from labels");
  await expect(row).toContainText("0 of 2 running");

  await page.getByRole("button", { name: `Compose Up ${COMPOSE_PROJECT}` }).click();
  await expect(row).toContainText("2 of 2 running", { timeout: 60_000 });
  expect(containerState(`${COMPOSE_PROJECT}-web-1`)).toBe("running");

  await page.getByRole("button", { name: `More actions for ${COMPOSE_PROJECT}` }).click();
  await page.getByRole("menuitem", { name: "Down", exact: true }).click();
  await page.getByRole("dialog").or(page.getByRole("alertdialog")).getByRole("button", { name: /Down/ }).click();

  // Down removes the containers. Docklet keeps the project it has run, now as "Not created".
  await expect.poll(() => containerState(`${COMPOSE_PROJECT}-web-1`), { timeout: 60_000 }).toBe("missing");
  await expect(row).toContainText("Not created");
  await expect(row).toContainText("Managed from local files");
});

test("creates a new Compose file and opens it in the editor", async ({ page }) => {
  const name = "docklet-test-new";
  const dir = `/tmp/${name}`;
  await openPage(page, "/compose", TCP_CONNECTION);

  await page.getByRole("button", { name: "New Compose File" }).first().click();
  const dialog = page.getByRole("dialog", { name: "New Compose File" });
  // The folder is typed: the native picker does not exist in server mode.
  await dialog.getByRole("textbox", { name: "Folder" }).fill(dir);
  await expect(dialog.getByRole("textbox", { name: "Project name" })).toHaveValue(name);
  await expect(dialog.locator(".cm-content")).toContainText(`name: ${name}`);

  await dialog.getByRole("button", { name: "Create and open" }).click();

  await expect(page).toHaveURL(new RegExp(`/compose/${name}$`));
  await expect(page.getByRole("tab", { name: "Compose file" })).toHaveAttribute("aria-selected", "true");
  await expect(page.locator(".cm-content")).toContainText(`name: ${name}`);

  // The file is where Compose runs, and creating it started nothing.
  expect(dockletSh(`cat ${dir}/compose.yaml`)).toContain(`name: ${name}`);
  expect(engineDocker("ps", "-a", "-q", "--filter", `label=com.docker.compose.project=${name}`)).toBe("");
});
