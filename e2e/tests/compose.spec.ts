import { expect, test } from "@playwright/test";
import { openPage, rowOf } from "../support/app";
import { containerState } from "../support/engine";
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
