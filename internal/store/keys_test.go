package store

import (
	"encoding/json"
	"os"
	"testing"
)

func TestExerciseKeyVectors(t *testing.T) {
	data, err := os.ReadFile("testdata/exercise_keys.json")
	if err != nil {
		t.Fatal(err)
	}
	var vectors []struct{ Name, Key string }
	if err := json.Unmarshal(data, &vectors); err != nil {
		t.Fatal(err)
	}
	for _, v := range vectors {
		if got := ExerciseKey(v.Name); got != v.Key {
			t.Errorf("ExerciseKey(%q) = %q, want %q", v.Name, got, v.Key)
		}
	}
}
