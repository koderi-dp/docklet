import { expect, test } from "@playwright/test";
import { openPage, rowOf } from "../support/app";
import { engineDocker } from "../support/engine";
import { REGISTRY_IMAGE, TCP_CONNECTION } from "../support/names";

test("pulls an image from a registry", async ({ page }) => {
  await openPage(page, "/images", TCP_CONNECTION);
  await expect(rowOf(page, "busybox:1.36")).toBeVisible();
  await expect(rowOf(page, REGISTRY_IMAGE)).toHaveCount(0);

  await page.getByRole("button", { name: "Pull Image" }).click();
  const dialog = page.getByRole("dialog", { name: "Pull image" });
  await dialog.getByLabel("Image reference").fill(REGISTRY_IMAGE);
  await dialog.getByRole("button", { name: "Pull", exact: true }).click();

  await expect(rowOf(page, REGISTRY_IMAGE)).toBeVisible({ timeout: 60_000 });
  expect(engineDocker("image", "inspect", "-f", "{{.Id}}", REGISTRY_IMAGE)).toMatch(/^sha256:/);
});
