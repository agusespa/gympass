import {
  addExercise,
  addSet,
  expect,
  fillSet,
  finishWorkout,
  logBodyWeight,
  newWorkout,
  openApp,
  serverData,
  test,
  uid,
} from "./helpers";

function isoDaysAgo(days: number): string {
  const d = new Date();
  d.setDate(d.getDate() - days);
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;
}

test("profile shows stats, records and per-exercise progress", async ({
  page,
  request,
}) => {
  const id = uid();
  const squat = `Squat ${id}`;

  // An earlier session, logged a week ago from another device.
  const now = Date.now();
  const [w, e, s] = [
    crypto.randomUUID(),
    crypto.randomUUID(),
    crypto.randomUUID(),
  ];
  const base = { updated_at: now, deleted: false };
  const res = await request.post("/api/sync", {
    data: {
      cursor: 0,
      changes: {
        workouts: [
          {
            ...base,
            id: w,
            name: `Legs ${id}`,
            date: isoDaysAgo(8),
            started_at: now,
            finished_at: now,
          },
        ],
        workout_exercises: [
          {
            ...base,
            id: e,
            workout_id: w,
            name: squat,
            position: 1,
            superset_id: null,
          },
        ],
        sets: [
          {
            ...base,
            id: s,
            workout_exercise_id: e,
            position: 1,
            reps: 5,
            weight_kg: 100,
            warmup: false,
            rest_seconds: 120,
            done: true,
          },
        ],
      },
    },
  });
  expect(res.ok()).toBe(true);
  await openApp(page);

  // Today's session beats it.
  await newWorkout(page, `Legs again ${id}`);
  await addExercise(page, squat);
  await fillSet(page, squat, 1, "105", "5");
  await finishWorkout(page);

  await expect(page.getByTestId("stat-week")).not.toHaveText("0");
  // The session from last week gives a finished week to average.
  await expect(page.getByTestId("stat-avg")).not.toHaveText("–");
  await expect(
    page.getByRole("region", { name: "Weekly volume" }),
  ).toBeVisible();
  const record = page
    .getByRole("list", { name: "Recent records" })
    .getByRole("link", { name: new RegExp(squat) });
  await expect(record).toContainText("105 kg × 5");
  await expect(record).toContainText("New");
  await expect(
    page
      .getByRole("list", { name: "Exercises" })
      .getByRole("link", { name: new RegExp(squat) }),
  ).toContainText("2 sessions · best 105 kg × 5");

  // The exercise screen charts both sessions and lists them newest first.
  await record.click();
  await expect(page).toHaveURL(
    new RegExp(`/exercises/${encodeURIComponent(squat.toLowerCase())}$`),
  );
  await expect(page.getByTestId("best-weight")).toContainText("105 kg");
  await expect(page.getByTestId("best-e1rm")).toHaveText("122.5 kg");
  await expect(page.getByRole("region", { name: "Est. 1RM" })).toBeVisible();
  await expect(page.locator("a.session")).toHaveCount(2);
  await expect(page.locator("a.session").first()).toContainText("105 kg × 5");

  // Settings: sync status and export.
  await page.goto("/profile");
  await page.getByRole("link", { name: "Settings" }).click();
  await expect(page.getByTestId("sync-status")).toHaveText("Up to date");
  await expect(page.getByTestId("pending")).toHaveText("0");
  const download = page.waitForEvent("download");
  await page.getByRole("button", { name: "Sets as CSV" }).click();
  const csv = await (await download).createReadStream();
  let body = "";
  for await (const chunk of csv) body += chunk;
  expect(body).toContain(`${squat},1,105,5,false,true`);
});

test("body weight counts toward volume for bodyweight exercises", async ({
  page,
  request,
}) => {
  const id = uid();
  const pullup = `Pull-up ${id}`;
  await logBodyWeight(page, request, 80);

  // Before the exercise is set to count body weight, only the added load counts.
  await newWorkout(page, `Pull ${id}`);
  await addExercise(page, pullup);
  await fillSet(page, pullup, 1, "", "10");
  await page.getByRole("button", { name: "Finish", exact: true }).click();
  await expect(
    page.getByText("Volume", { exact: true }).locator(".."),
  ).toHaveText(/^\s*Volume\s*0 kg\s*$/);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page).toHaveURL("/profile");

  await page.goto(`/exercises/${encodeURIComponent(pullup.toLowerCase())}`);
  await page.getByLabel("Body weight", { exact: true }).selectOption("100");
  await expect
    .poll(async () =>
      ((await serverData(request)).exercise_settings ?? []).some(
        (x) => x.name === pullup && x.bodyweight_pct === 100,
      ),
    )
    .toBe(true);

  // Now each rep moves 80 kg of body weight plus any added load.
  await newWorkout(page, `Pull again ${id}`);
  await addExercise(page, pullup);
  await fillSet(page, pullup, 1, "", "10");
  await addSet(page, pullup);
  await fillSet(page, pullup, 2, "10", "5");
  await page.getByRole("button", { name: "Finish", exact: true }).click();
  await expect(
    page.getByText("Volume", { exact: true }).locator(".."),
  ).toContainText(`${10 * 80 + 5 * 90} kg`);
});

test("weekly goal syncs and flags weeks that fall short", async ({
  page,
  request,
}) => {
  // Set on another device; the settings row is shared, so this test puts it back to Off.
  const res = await request.post("/api/sync", {
    data: {
      cursor: 0,
      changes: {
        settings: [
          {
            id: "user",
            updated_at: Date.now(),
            deleted: false,
            weekly_goal: 2,
          },
        ],
      },
    },
  });
  expect(res.ok()).toBe(true);
  await openApp(page, "/profile/settings");
  const goal = page.getByTestId("weekly-goal");
  await expect(goal).toHaveText("2");

  await page.getByRole("button", { name: "More workouts per week" }).click();
  await expect(goal).toHaveText("3");
  await expect
    .poll(
      async () =>
        (await serverData(request)).settings?.find((x) => x.id === "user")
          ?.weekly_goal,
    )
    .toBe(3);

  await page.goto("/profile");
  await expect(page.getByTestId("stat-week")).toContainText("/ 3");
  const chart = page.getByRole("region", { name: "Weekly volume" });
  await expect(
    chart.getByRole("button", { name: /goal missed/ }).first(),
  ).toBeVisible();
  // The stat tile already shows this week's count, so the chart only shows it for past weeks.
  await expect(chart.getByTestId("week-workouts")).toHaveCount(0);
  await chart
    .getByRole("button", { name: /goal missed/ })
    .first()
    .click();
  await expect(chart.getByTestId("week-workouts")).toContainText(
    "/ 3 workouts",
  );

  await page.getByRole("link", { name: "Settings" }).click();
  const fewer = page.getByRole("button", { name: "Fewer workouts per week" });
  for (let i = 0; i < 3; i++) await fewer.click();
  await expect(goal).toHaveText("Off");
  await expect(fewer).toBeDisabled();
  await expect
    .poll(
      async () =>
        (await serverData(request)).settings?.find((x) => x.id === "user")
          ?.weekly_goal,
    )
    .toBe(0);

  await page.goto("/profile");
  await expect(page.getByTestId("stat-week")).not.toContainText("/");
  await expect(chart.getByRole("button", { name: /goal missed/ })).toHaveCount(
    0,
  );
});
