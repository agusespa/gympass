package store

import (
	_ "embed"
	"encoding/json"
	"slices"
	"sort"
	"strings"
	"unicode"
)

// catalog.json lists every exercise there is; the app imports the same file
// (web/src/lib/catalog.ts). Workout and routine rows refer to an entry by its
// id.
//
//go:embed catalog.json
var catalogJSON []byte

// Exercise is a catalogue entry. Measure is what a set of it records: reps,
// or time (duration_seconds, e.g. a dead hang).
type Exercise struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Measure   string `json:"measure"`
	Equipment string `json:"equipment"`
}

// Mirror Measure and Equipment in web/src/lib/catalog.ts.
var (
	measures   = []string{"reps", "time"}
	equipments = []string{"barbell", "dumbbell", "machine", "plate", "cable", "kettlebell", "bodyweight"}
)

var (
	catalog      []Exercise
	catalogByID  = map[string]Exercise{}
	catalogByKey = map[string]Exercise{} // by ExerciseKey of the name
)

func init() {
	if err := json.Unmarshal(catalogJSON, &catalog); err != nil {
		panic("catalog.json: " + err.Error())
	}
	for _, e := range catalog {
		catalogByID[e.ID] = e
		catalogByKey[ExerciseKey(e.Name)] = e
	}
}

// exerciseName is the name of the catalogue entry with this id, "" if there is none.
func exerciseName(id string) string { return catalogByID[id].Name }

// Exercises returns the catalogue sorted by name.
func Exercises() []Exercise {
	out := slices.Clone(catalog)
	sort.Slice(out, func(i, j int) bool { return ExerciseKey(out[i].Name) < ExerciseKey(out[j].Name) })
	return out
}

// ExerciseKey is how the API matches an exercise name to the catalogue:
// trimmed and lowercased one rune at a time.
func ExerciseKey(name string) string {
	return strings.ToLower(TrimName(name))
}

// TrimName strips leading and trailing white space: Unicode White_Space plus
// U+FEFF, the same set as the client's trimName (JS \s plus U+0085).
func TrimName(name string) string {
	return strings.TrimFunc(name, func(r rune) bool { return unicode.IsSpace(r) || r == '\uFEFF' })
}
