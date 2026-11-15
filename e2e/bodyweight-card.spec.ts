import {
  clearBodyWeights,
  expect,
  logBodyWeight,
  openApp,
  test,
} from "./helpers";

test("profile asks for a body weight until one is logged", async ({
  page,
  request,
}) => {
  await clearBodyWeights(request);
  await openApp(page, "/profile");
  const card = page.getByRole("region", { name: "Add your body weight" });
  await card.getByRole("link").click();
  await expect(page).toHaveURL("/profile/settings");

  await logBodyWeight(page, request, 80);
  await page.goto("/profile");
  await expect(page.getByTestId("stat-avg")).toBeVisible();
  await expect(card).toHaveCount(0);
});

test("the body weight card can be dismissed", async ({ page, request }) => {
  await clearBodyWeights(request);
  await openApp(page, "/profile");
  const card = page.getByRole("region", { name: "Add your body weight" });
  await card.getByRole("button", { name: "Dismiss" }).click();
  await expect(card).toHaveCount(0);
  await page.reload();
  await expect(page.getByTestId("stat-avg")).toBeVisible();
  await expect(card).toHaveCount(0);
});
