import { expect, newWorkout, openApp, serverData, test, uid } from "./helpers";

test("a row the server refuses does not block the rest of sync", async ({
  page,
  request,
}) => {
  const id = uid();
  await openApp(page);

  // A dirty set the server will refuse (over the weight limit), as an older
  // build without input limits could have stored.
  await page.evaluate(
    (id) =>
      new Promise<void>((resolve, reject) => {
        const open = indexedDB.open("gym-tracker");
        open.onerror = () => reject(open.error);
        open.onsuccess = () => {
          const tx = open.result.transaction("sets", "readwrite");
          tx.objectStore("sets").put({
            id: `bad-${id}`,
            updated_at: Date.now(),
            deleted: false,
            dirty: 1,
            workout_exercise_id: `ex-${id}`,
            position: 1,
            reps: 5,
            weight_kg: 20000,
            warmup: false,
            rest_seconds: 120,
            done: true,
            duration_seconds: 0,
            rep_min: 0,
            rep_max: 0,
          });
          tx.oncomplete = () => resolve();
          tx.onerror = () => reject(tx.error);
        };
      }),
    id,
  );

  await newWorkout(page, `Kept ${id}`);
  await expect(page.getByRole("alert")).toContainText(
    "1 change couldn't be saved to the server",
  );
  await expect
    .poll(async () =>
      (await serverData(request)).workouts?.some(
        (w) => w.name === `Kept ${id}`,
      ),
    )
    .toBe(true);
  expect(
    (await serverData(request)).sets?.some((s) => s.id === `bad-${id}`),
  ).toBeFalsy();
});
