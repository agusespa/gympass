import { readFileSync } from "node:fs";
import { expect, test } from "@playwright/test";
import { exerciseKey } from "../web/src/lib/exercise-key.ts";

// The server checks exercise_settings ids with its own ExerciseKey, tested
// against the same file (internal/store TestExerciseKeyVectors).
const vectors: { name: string; key: string }[] = JSON.parse(
  readFileSync(
    new URL("../internal/store/testdata/exercise_keys.json", import.meta.url),
    "utf8",
  ),
);

test("exercise keys match the server", () => {
  for (const v of vectors)
    expect(exerciseKey(v.name), JSON.stringify(v.name)).toBe(v.key);
});
