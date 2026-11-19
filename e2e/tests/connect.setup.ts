import { expect, test } from "@playwright/test";
import { TCP_CONNECTION } from "../support/names";

test("first run: skip setup and add the engine over TCP", async ({ page }) => {
  await page.goto("/");

  // An empty profile starts in first-time setup.
  await expect(page.getByRole("heading", { name: "Welcome to Docklet" })).toBeVisible();
  await page.getByRole("button", { name: "Skip setup" }).click();
  await page.getByRole("alertdialog").getByRole("button", { name: "Skip setup" }).click();

  // Without a reachable engine the app points to the Connections page.
  await page.getByRole("button", { name: "Connections" }).click();
  await page.getByRole("button", { name: "Add connection" }).click();
  const dialog = page.getByRole("dialog");
  await dialog.getByRole("radio", { name: /TCP\/TLS/ }).click();
  await dialog.getByLabel("Name").fill(TCP_CONNECTION);
  await dialog.getByLabel("Host").fill("engine");
  await dialog.getByLabel("Secure with TLS").click();
  await expect(dialog.getByLabel("Port")).toHaveValue("2375");
  // Plain TCP has to be confirmed before it can be saved.
  await expect(dialog.getByRole("button", { name: "Test & Save" })).toBeDisabled();
  await dialog.getByRole("checkbox", { name: /Allow insecure plain TCP/ }).check();
  await dialog.getByRole("button", { name: "Test & Save" }).click();

  await expect(page.getByText(`Added ${TCP_CONNECTION}`)).toBeVisible();
});
