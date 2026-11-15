import {
  addExercise,
  addSet,
  exercise,
  exerciseMenu,
  expect,
  newWorkout,
  serverData,
  setText,
  test,
  uid,
} from "./helpers";

test("log a timed exercise such as a dead hang", async ({ page, request }) => {
  const id = uid();
  const hang = `Dead hang ${id}`;

  await newWorkout(page, `Grip ${id}`);
  await addExercise(page, hang);
  const ex = exercise(page, hang);
  await exerciseMenu(page, hang, "Log time");

  // Seconds or m:ss, shown as m:ss.
  await setText(ex.getByLabel("Time set 1"), "45");
  await expect(ex.getByLabel("Time set 1")).toHaveValue("0:45");
  await expect(ex.getByLabel("Reps set 1")).toHaveCount(0);
  await ex.getByRole("button", { name: "Complete set 1", exact: true }).click();
  await addSet(page, hang);
  await setText(ex.getByLabel("Time set 2"), "1:10");
  await expect(ex.getByLabel("Time set 2")).toHaveValue("1:10");
  await ex.getByRole("button", { name: "Complete set 2", exact: true }).click();

  await page.getByRole("button", { name: "Finish", exact: true }).click();
  await expect(
    page.getByRole("list", { name: "Exercises done" }),
  ).toContainText(`${hang} 2 sets · best 1:10`);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(page).toHaveURL("/profile");

  await expect
    .poll(async () => {
      const data = await serverData(request);
      const exId = data.workout_exercises?.find((e) => e.name === hang)?.id;
      return (data.sets ?? [])
        .filter((s) => s.workout_exercise_id === exId && !s.deleted)
        .map((s) => s.duration_seconds)
        .sort();
    })
    .toEqual([45, 70]);

  // Next time starts from these times and shows them as the previous values.
  await newWorkout(page, `Grip again ${id}`);
  await addExercise(page, hang);
  const again = exercise(page, hang);
  await expect(again.getByTestId("previous")).toHaveText(["0:45", "1:10"]);
  await expect(again.getByLabel("Time set 2")).toHaveValue("1:10");
});
