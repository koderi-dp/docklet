import { expect, test } from "@playwright/test";
import { openPage, rowAction, rowOf } from "../support/app";
import { containerState } from "../support/engine";
import { DOOMED_CONTAINER, LOG_CONTAINER, TCP_CONNECTION, TOGGLE_CONTAINER } from "../support/names";

test.beforeEach(async ({ page }) => {
  await openPage(page, "/containers", TCP_CONNECTION);
});

test("lists the containers of the engine with their state", async ({ page }) => {
  await expect(rowOf(page, LOG_CONTAINER)).toContainText("Running");
  await expect(rowOf(page, LOG_CONTAINER)).toContainText("busybox:1.36");
  await expect(rowOf(page, TOGGLE_CONTAINER)).toContainText("Running");
  await expect(page.getByRole("row", { name: /Standalone \d+ containers/ })).toBeVisible();
});

test("stops and starts a container", async ({ page }) => {
  const row = rowOf(page, TOGGLE_CONTAINER);
  await expect(row).toContainText("Running");

  await rowAction(page, TOGGLE_CONTAINER, "Stop");
  await expect(row).toContainText("Exited", { timeout: 30_000 });
  expect(containerState(TOGGLE_CONTAINER)).toBe("exited");

  await rowAction(page, TOGGLE_CONTAINER, "Start");
  await expect(row).toContainText("Running");
  expect(containerState(TOGGLE_CONTAINER)).toBe("running");
});

test("streams the logs of a running container", async ({ page }) => {
  await page.getByRole("cell", { name: LOG_CONTAINER, exact: true }).click();
  const inspector = page.getByRole("complementary", { name: `Inspector for ${LOG_CONTAINER}` });
  await inspector.getByRole("tab", { name: "Logs" }).click();

  const log = inspector.getByRole("log", { name: `Logs of ${LOG_CONTAINER}` });
  await expect(log).toContainText("tick 1");

  // The container prints a line every second: a later line must arrive without a reload.
  const latest = async () => Math.max(...[...(await log.innerText()).matchAll(/tick (\d+)/g)].map((m) => Number(m[1])));
  const before = await latest();
  await expect.poll(latest, { timeout: 15_000 }).toBeGreaterThan(before + 1);
});

test("deletes a container after confirmation", async ({ page }) => {
  await expect(rowOf(page, DOOMED_CONTAINER)).toBeVisible();

  await rowAction(page, DOOMED_CONTAINER, "Delete…");
  const confirm = page.getByRole("alertdialog");
  await expect(confirm).toContainText(DOOMED_CONTAINER);
  await confirm.getByRole("button", { name: /^(Delete|Remove)/ }).click();

  await expect.poll(() => containerState(DOOMED_CONTAINER), { timeout: 30_000 }).toBe("missing");
  await expect(rowOf(page, DOOMED_CONTAINER)).toHaveCount(0);
});
