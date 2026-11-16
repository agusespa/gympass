package store

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"unicode/utf8"
)

// Routine is a routine with its live exercises and sets, in order. It is the
// shape both read and written by the routine API.
type Routine struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Exercises []RoutineExercise `json:"exercises"`
}

type RoutineExercise struct {
	ID string `json:"id"`
	// ExerciseID is the catalogue entry's id, set on read. On write the
	// exercise is chosen by Name, which must be a catalogue entry's.
	ExerciseID string `json:"exercise_id"`
	Name       string `json:"name"`
	// SupersetID groups adjacent exercises into a superset. On write it is a
	// label: adjacent exercises sharing one form a superset, a lone one is
	// cleared, and a label that is not already this routine's superset id is
	// replaced by a new id.
	SupersetID *string      `json:"superset_id"`
	Sets       []RoutineSet `json:"sets"`
}

type RoutineSet struct {
	ID          string  `json:"id"`
	Reps        int64   `json:"reps"`
	WeightKg    float64 `json:"weight_kg"`
	Warmup      bool    `json:"warmup"`
	RestSeconds int64   `json:"rest_seconds"`
	// DurationSeconds is the planned length of a timed set (e.g. a dead
	// hang); 0 for sets counted in reps.
	DurationSeconds int64 `json:"duration_seconds"`
	// RepMin and RepMax are the target rep range, e.g. 8 to 12; both 0 for
	// a fixed rep count. With a range, Reps equals RepMin.
	RepMin int64 `json:"rep_min"`
	RepMax int64 `json:"rep_max"`
}

// ErrNotFound means the routine does not exist or was deleted.
var ErrNotFound = errors.New("not found")

const (
	maxRoutineExercises = 50
	maxExerciseSets     = 50
)

// RoutinePage returns up to limit live routines sorted by name (ignoring
// ASCII case), then id, starting after the routine named afterName with id
// afterID (afterID "" starts at the first). more is true when routines remain.
func (s *Store) RoutinePage(ctx context.Context, afterName, afterID string, limit int) (page []Routine, more bool, err error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, false, err
	}
	defer func() { _ = tx.Rollback() }()
	rs, err := tx.QueryContext(ctx, `SELECT id FROM routines WHERE deleted = 0
  AND (?2 = '' OR name COLLATE NOCASE > ?1 OR (name COLLATE NOCASE = ?1 AND id > ?2))
ORDER BY name COLLATE NOCASE, id LIMIT ?3`, afterName, afterID, limit+1)
	if err != nil {
		return nil, false, err
	}
	var ids []string
	for rs.Next() {
		var id string
		if err := rs.Scan(&id); err != nil {
			_ = rs.Close()
			return nil, false, err
		}
		ids = append(ids, id)
	}
	err = rs.Err()
	if cerr := rs.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, false, err
	}
	if len(ids) > limit {
		ids, more = ids[:limit], true
	}
	page = make([]Routine, 0, len(ids))
	for _, id := range ids {
		cur, err := loadRoutine(ctx, tx, id)
		if err != nil {
			return nil, false, err
		}
		if cur != nil {
			page = append(page, cur.routine())
		}
	}
	return page, more, nil
}

// Routine returns one live routine, or ok=false.
func (s *Store) Routine(ctx context.Context, id string) (Routine, bool, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return Routine{}, false, err
	}
	defer func() { _ = tx.Rollback() }()
	cur, err := loadRoutine(ctx, tx, id)
	if err != nil || cur == nil {
		return Routine{}, false, err
	}
	return cur.routine(), true, nil
}

// CreateRoutine stores r as a new routine. Ids in r are ignored; new ones are
// generated.
func (s *Store) CreateRoutine(ctx context.Context, r Routine) (Routine, error) {
	r.ID = newID()
	for i := range r.Exercises {
		r.Exercises[i].ID = ""
		for j := range r.Exercises[i].Sets {
			r.Exercises[i].Sets[j].ID = ""
		}
	}
	return s.saveRoutine(ctx, r, true)
}

// ReplaceRoutine makes routine r.ID look exactly like r. Exercises and sets
// with an id keep it (the id must belong to this routine, and a set's id to
// that exercise); those without one are created; existing ones left out are
// deleted. Only rows that actually change are written, so phones receive them
// through sync like any other edit. Returns ErrNotFound for an unknown routine.
func (s *Store) ReplaceRoutine(ctx context.Context, r Routine) (Routine, error) {
	return s.saveRoutine(ctx, r, false)
}

func (s *Store) saveRoutine(ctx context.Context, r Routine, create bool) (Routine, error) {
	if err := validateRoutine(&r); err != nil {
		return Routine{}, err
	}
	if err := resolveExercises(&r); err != nil {
		return Routine{}, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return Routine{}, err
	}
	defer func() { _ = tx.Rollback() }()

	cur := &storedRoutine{}
	if !create {
		if cur, err = loadRoutine(ctx, tx, r.ID); err != nil {
			return Routine{}, err
		}
		if cur == nil {
			return Routine{}, ErrNotFound
		}
	}
	w := &rowWriter{now: s.nowMs()}
	if err := w.plan(cur, r); err != nil {
		return Routine{}, err
	}
	if err := w.flush(ctx, tx); err != nil {
		return Routine{}, err
	}
	saved, err := loadRoutine(ctx, tx, r.ID)
	if err != nil {
		return Routine{}, err
	}
	if err := tx.Commit(); err != nil {
		return Routine{}, err
	}
	return saved.routine(), nil
}

func validateRoutine(r *Routine) error {
	text := func(what, s string) (string, error) {
		s = strings.TrimSpace(s)
		if s == "" || utf8.RuneCountInString(s) > maxTextLen {
			return "", invalid("%s must be 1 to %d characters", what, maxTextLen)
		}
		return s, nil
	}
	var err error
	if r.Name, err = text("name", r.Name); err != nil {
		return err
	}
	if len(r.Exercises) > maxRoutineExercises {
		return invalid("at most %d exercises", maxRoutineExercises)
	}
	ids := map[string]bool{}
	seen := func(what, id string) error {
		if id == "" {
			return nil
		}
		if len(id) > maxIDLen {
			return invalid("%s id is longer than %d bytes", what, maxIDLen)
		}
		if ids[id] {
			return invalid("%s id %q is used twice", what, id)
		}
		ids[id] = true
		return nil
	}
	for i := range r.Exercises {
		e := &r.Exercises[i]
		where := fmt.Sprintf("exercises[%d]", i)
		if e.Name, err = text(where+".name", e.Name); err != nil {
			return err
		}
		if err := seen(where, e.ID); err != nil {
			return err
		}
		if e.SupersetID != nil && len(*e.SupersetID) > maxIDLen {
			return invalid("%s.superset_id is longer than %d bytes", where, maxIDLen)
		}
		if len(e.Sets) > maxExerciseSets {
			return invalid("%s: at most %d sets", where, maxExerciseSets)
		}
		for j, st := range e.Sets {
			where := fmt.Sprintf("%s.sets[%d]", where, j)
			if err := seen(where, st.ID); err != nil {
				return err
			}
			switch {
			case st.Reps < 0 || st.Reps > maxReps:
				return invalid("%s.reps must be 0 to %d", where, maxReps)
			case math.IsNaN(st.WeightKg) || st.WeightKg < 0 || st.WeightKg > maxWeightKg:
				return invalid("%s.weight_kg must be 0 to %d", where, maxWeightKg)
			case st.RestSeconds < 0 || st.RestSeconds > maxRestSeconds:
				return invalid("%s.rest_seconds must be 0 to %d", where, maxRestSeconds)
			case st.DurationSeconds < 0 || st.DurationSeconds > maxDurationSeconds:
				return invalid("%s.duration_seconds must be 0 to %d", where, maxDurationSeconds)
			case st.RepMax < 0 || st.RepMax > maxReps:
				return invalid("%s.rep_max must be 0 to %d", where, maxReps)
			}
			if st.RepMax == 0 {
				e.Sets[j].RepMin = 0
			} else {
				// The range is a guide: reps may reach or pass the max.
				if st.RepMin < 0 || st.RepMin > maxReps {
					return invalid("%s.rep_min must be 0 to %d", where, maxReps)
				}
				e.Sets[j].Reps = st.RepMin
			}
		}
	}
	return nil
}

// storedRoutine is a routine's live rows as they are in the database.
type storedRoutine struct {
	id, name  string
	updatedAt int64
	exercises []storedExercise
}

type storedExercise struct {
	id, exerciseID, name string
	supersetID           *string
	position             float64
	updatedAt            int64
	sets                 []storedSet
}

type storedSet struct {
	id          string
	position    float64
	reps        int64
	weightKg    float64
	warmup      bool
	restSeconds int64
	duration    int64
	repMin      int64
	repMax      int64
	updatedAt   int64
}

func (c *storedRoutine) routine() Routine {
	r := Routine{ID: c.id, Name: c.name, Exercises: make([]RoutineExercise, 0, len(c.exercises))}
	for _, e := range c.exercises {
		ex := RoutineExercise{ID: e.id, ExerciseID: e.exerciseID, Name: e.name, SupersetID: e.supersetID, Sets: make([]RoutineSet, 0, len(e.sets))}
		for _, st := range e.sets {
			ex.Sets = append(ex.Sets, RoutineSet{
				ID: st.id, Reps: st.reps, WeightKg: st.weightKg, Warmup: st.warmup, RestSeconds: st.restSeconds,
				DurationSeconds: st.duration, RepMin: st.repMin, RepMax: st.repMax,
			})
		}
		r.Exercises = append(r.Exercises, ex)
	}
	return r
}

// loadRoutine returns nil if the routine is missing or deleted.
func loadRoutine(ctx context.Context, tx *sql.Tx, id string) (*storedRoutine, error) {
	c := &storedRoutine{id: id}
	err := tx.QueryRowContext(ctx, "SELECT name, updated_at FROM routines WHERE id = ? AND deleted = 0", id).
		Scan(&c.name, &c.updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	rs, err := tx.QueryContext(ctx, `SELECT e.id, e.exercise_id, e.superset_id, e.position, e.updated_at,
  s.id, s.position, s.reps, s.weight_kg, s.warmup, s.rest_seconds, s.duration_seconds, s.rep_min, s.rep_max, s.updated_at
FROM routine_exercises e
LEFT JOIN routine_sets s ON s.routine_exercise_id = e.id AND s.deleted = 0
WHERE e.routine_id = ? AND e.deleted = 0
ORDER BY e.position, e.id, s.position, s.id`, id)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rs.Close() }()
	for rs.Next() {
		var e storedExercise
		var superset, setID sql.NullString
		var setPos, weight sql.NullFloat64
		var reps, warmup, rest, duration, repMin, repMax, setUpdated sql.NullInt64
		if err := rs.Scan(&e.id, &e.exerciseID, &superset, &e.position, &e.updatedAt,
			&setID, &setPos, &reps, &weight, &warmup, &rest, &duration, &repMin, &repMax, &setUpdated); err != nil {
			return nil, err
		}
		if n := len(c.exercises); n == 0 || c.exercises[n-1].id != e.id {
			e.name = exerciseName(e.exerciseID)
			if superset.Valid {
				e.supersetID = &superset.String
			}
			c.exercises = append(c.exercises, e)
		}
		if setID.Valid {
			ex := &c.exercises[len(c.exercises)-1]
			ex.sets = append(ex.sets, storedSet{
				id: setID.String, position: setPos.Float64, reps: reps.Int64, weightKg: weight.Float64,
				warmup: warmup.Int64 == 1, restSeconds: rest.Int64, duration: duration.Int64,
				repMin: repMin.Int64, repMax: repMax.Int64, updatedAt: setUpdated.Int64,
			})
		}
	}
	return c, rs.Err()
}

// rowWriter collects the rows a routine save has to write.
type rowWriter struct {
	now  int64
	rows map[*table][]Row
}

// stamp picks an updated_at that beats the stored one even if a phone's clock
// ran ahead of the server's, so the write is never silently dropped.
func (w *rowWriter) stamp(old int64) int64 { return max(w.now, old+1) }

func (w *rowWriter) add(name string, r Row) {
	if w.rows == nil {
		w.rows = map[*table][]Row{}
	}
	t := tableByName[name]
	w.rows[t] = append(w.rows[t], r)
}

// resolveExercises sets each exercise's ExerciseID and Name from the
// catalogue entry with that name; any other name is refused.
func resolveExercises(r *Routine) error {
	for i := range r.Exercises {
		e := &r.Exercises[i]
		c, ok := catalogByKey[ExerciseKey(e.Name)]
		if !ok {
			return invalid("exercises[%d].name %q is not a known exercise", i, e.Name)
		}
		e.ExerciseID, e.Name = c.ID, c.Name
	}
	return nil
}

func (w *rowWriter) plan(cur *storedRoutine, r Routine) error {
	if cur.updatedAt == 0 || cur.name != r.Name {
		w.add("routines", Row{"id": r.ID, "updated_at": w.stamp(cur.updatedAt), "deleted": false, "name": r.Name})
	}

	oldEx := map[string]*storedExercise{}
	for i := range cur.exercises {
		oldEx[cur.exercises[i].id] = &cur.exercises[i]
	}
	supersets := supersetIDs(cur, r.Exercises)
	exPos := make([]*float64, len(r.Exercises))
	for i, e := range r.Exercises {
		if o, ok := oldEx[e.ID]; ok && e.ID != "" {
			exPos[i] = &o.position
		}
	}
	exPositions := positions(exPos)
	keptEx := map[string]bool{}
	for i, e := range r.Exercises {
		old := &storedExercise{}
		if e.ID != "" {
			o, ok := oldEx[e.ID]
			if !ok {
				return invalid("exercises[%d].id %q is not an exercise of this routine", i, e.ID)
			}
			old, keptEx[e.ID] = o, true
		} else {
			e.ID = newID()
		}
		pos := exPositions[i]
		superset := supersets[i]
		if old.updatedAt == 0 || old.exerciseID != e.ExerciseID || old.position != pos || !sameString(old.supersetID, superset) {
			w.add("routine_exercises", Row{
				"id": e.ID, "updated_at": w.stamp(old.updatedAt), "deleted": false,
				"routine_id": r.ID, "exercise_id": e.ExerciseID, "position": pos, "superset_id": nullable(superset),
			})
		}

		oldSets := map[string]*storedSet{}
		for j := range old.sets {
			oldSets[old.sets[j].id] = &old.sets[j]
		}
		setPos := make([]*float64, len(e.Sets))
		for j, st := range e.Sets {
			if o, ok := oldSets[st.ID]; ok && st.ID != "" {
				setPos[j] = &o.position
			}
		}
		setPositions := positions(setPos)
		keptSets := map[string]bool{}
		for j, st := range e.Sets {
			os := &storedSet{}
			if st.ID != "" {
				o, ok := oldSets[st.ID]
				if !ok {
					return invalid("exercises[%d].sets[%d].id %q is not a set of that exercise", i, j, st.ID)
				}
				os, keptSets[st.ID] = o, true
			} else {
				st.ID = newID()
			}
			pos := setPositions[j]
			if os.updatedAt == 0 || os.position != pos || os.reps != st.Reps || os.weightKg != st.WeightKg ||
				os.warmup != st.Warmup || os.restSeconds != st.RestSeconds || os.duration != st.DurationSeconds ||
				os.repMin != st.RepMin || os.repMax != st.RepMax {
				w.add("routine_sets", Row{
					"id": st.ID, "updated_at": w.stamp(os.updatedAt), "deleted": false,
					"routine_exercise_id": e.ID, "position": pos, "reps": st.Reps, "weight_kg": st.WeightKg,
					"warmup": st.Warmup, "rest_seconds": st.RestSeconds, "duration_seconds": st.DurationSeconds,
					"rep_min": st.RepMin, "rep_max": st.RepMax,
				})
			}
		}
		for _, os := range old.sets {
			if !keptSets[os.id] {
				w.deleteSet(e.ID, os)
			}
		}
	}
	for _, old := range cur.exercises {
		if keptEx[old.id] {
			continue
		}
		w.add("routine_exercises", Row{
			"id": old.id, "updated_at": w.stamp(old.updatedAt), "deleted": true,
			"routine_id": r.ID, "exercise_id": old.exerciseID, "position": old.position, "superset_id": nullable(old.supersetID),
		})
		for _, os := range old.sets {
			w.deleteSet(old.id, os)
		}
	}
	return nil
}

func (w *rowWriter) deleteSet(exerciseID string, s storedSet) {
	w.add("routine_sets", Row{
		"id": s.id, "updated_at": w.stamp(s.updatedAt), "deleted": true,
		"routine_exercise_id": exerciseID, "position": s.position, "reps": s.reps, "weight_kg": s.weightKg,
		"warmup": s.warmup, "rest_seconds": s.restSeconds, "duration_seconds": s.duration,
		"rep_min": s.repMin, "rep_max": s.repMax,
	})
}

// flush writes the planned rows under one new seq, as Sync does.
func (w *rowWriter) flush(ctx context.Context, tx *sql.Tx) error {
	rows := make(map[*table][][]any, len(w.rows))
	for t, list := range w.rows {
		for _, r := range list {
			args, err := t.args(r, w.now)
			if err != nil {
				return fmt.Errorf("%s row: %w", t.name, err)
			}
			rows[t] = append(rows[t], args)
		}
	}
	_, _, err := writeRows(ctx, tx, rows)
	return err
}

// supersetIDs turns the requested superset labels into ids, following the
// app's rule that a superset is a run of two or more adjacent exercises
// sharing an id. A run reuses its label when that is already one of this
// routine's superset ids (and no earlier run took it); otherwise it gets a
// new id. Exercises not in such a run get nil.
func supersetIDs(cur *storedRoutine, exercises []RoutineExercise) []*string {
	existing := map[string]bool{}
	for _, e := range cur.exercises {
		if e.supersetID != nil {
			existing[*e.supersetID] = true
		}
	}
	used := map[string]bool{}
	out := make([]*string, len(exercises))
	for i := 0; i < len(exercises); {
		label := exercises[i].SupersetID
		j := i + 1
		for label != nil && *label != "" && j < len(exercises) && sameString(exercises[j].SupersetID, label) {
			j++
		}
		if j-i >= 2 {
			id := *label
			if !existing[id] || used[id] {
				id = newID()
			}
			used[id] = true
			for k := i; k < j; k++ {
				out[k] = &id
			}
		}
		i = j
	}
	return out
}

// positions orders items by keeping as many stored positions (nil for a new
// item) as still increase, and placing the rest evenly between their kept
// neighbours, as the app does. Reordering or inserting
// therefore rewrites only the rows that moved.
func positions(old []*float64) []float64 {
	out := make([]float64, len(old))
	kept := keepIncreasing(old)
	for i, p := range old {
		if kept[i] {
			out[i] = *p
		}
	}
	for i := 0; i < len(old); {
		if kept[i] {
			i++
			continue
		}
		j := i
		for j < len(old) && !kept[j] {
			j++
		}
		n := j - i
		var lo, hi float64
		switch {
		case i > 0 && j < len(old):
			lo, hi = out[i-1], out[j]
		case i > 0:
			lo = out[i-1]
			hi = lo + float64(n) + 1
		case j < len(old):
			hi = out[j]
			lo = hi - float64(n) - 1
		default:
			lo, hi = 0, float64(n)+1
		}
		step := (hi - lo) / float64(n+1)
		for k := 0; k < n; k++ {
			out[i+k] = lo + step*float64(k+1)
		}
		i = j
	}
	return out
}

// keepIncreasing marks the longest strictly increasing run (not necessarily
// adjacent) of stored positions, so the fewest rows have to move.
func keepIncreasing(old []*float64) []bool {
	n := len(old)
	length := make([]int, n) // longest increasing chain ending at i
	prev := make([]int, n)
	best := -1
	for i, p := range old {
		prev[i] = -1
		if p == nil {
			continue
		}
		length[i] = 1
		for k := 0; k < i; k++ {
			if old[k] != nil && *old[k] < *p && length[k]+1 > length[i] {
				length[i], prev[i] = length[k]+1, k
			}
		}
		if best < 0 || length[i] > length[best] {
			best = i
		}
	}
	kept := make([]bool, n)
	for i := best; i >= 0; i = prev[i] {
		kept[i] = true
	}
	return kept
}

func sameString(a, b *string) bool {
	if a == nil || b == nil {
		return a == b
	}
	return *a == *b
}

func nullable(s *string) any {
	if s == nil {
		return nil
	}
	return *s
}

// newID returns a random UUID v4, the same form the app generates.
func newID() string {
	var b [16]byte
	_, _ = rand.Read(b[:])
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}
