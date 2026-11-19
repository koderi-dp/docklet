import { expect, type Locator, type Page } from "@playwright/test";

/** Opens a page of the app by its route, e.g. "/containers", with `connection` as the active engine. */
export async function openPage(page: Page, route: string, connection: string) {
  await page.goto(`/#${route}`);
  await useConnection(page, connection);
}

/** Makes `name` the active engine through the picker in the top bar and waits until it is connected. */
export async function useConnection(page: Page, name: string) {
  const picker = page.getByRole("button", { name: "Active engine" });
  await expect(picker).toBeVisible();
  if (!(await picker.innerText()).includes(name)) {
    await picker.click();
    await page.getByRole("menuitem", { name }).click();
  }
  await expect(picker).toContainText(name);
  await expect(page.getByRole("contentinfo")).toContainText(`Connected: ${name}`, { timeout: 30_000 });
}

/**
 * The table row of a container, image or Compose project. Rows are found by their actions button
 * ("Actions for web-1"), whose name stays the same while the row shows a spinner or a new state.
 */
export function rowOf(page: Page, name: string): Locator {
  const actions = new RegExp(`^(More actions|Actions) for ${escapeRegExp(name)}$`);
  return page.getByRole("row").filter({ has: page.getByRole("button", { name: actions }) });
}

function escapeRegExp(s: string): string {
  return s.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
}

/** Opens the "Actions for …" menu of a container or image row and picks an item. */
export async function rowAction(page: Page, name: string, item: string) {
  await page.getByRole("button", { name: `Actions for ${name}`, exact: true }).click();
  await page.getByRole("menuitem", { name: item, exact: true }).click();
}
