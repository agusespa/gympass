/**
 * How exercise names are matched. Mirrors TrimName and ExerciseKey in
 * internal/store/webhook.go; the server derives exercise_settings ids with
 * them, so any difference makes it refuse the row. Both sides are tested
 * against internal/store/testdata/exercise_keys.json.
 */

/** Leading and trailing white space: JS \s plus U+0085, which Go also trims. */
const EDGE_SPACE = /^[\s\u0085]+|[\s\u0085]+$/g;

export function trimName(name: string): string {
	return name.replace(EDGE_SPACE, '');
}

/**
 * Trimmed and lowercased one code point at a time, like Go's strings.ToLower:
 * no context rules (final sigma), and İ maps to a plain i.
 */
export function exerciseKey(name: string): string {
	let key = '';
	for (const c of trimName(name)) key += c === 'İ' ? 'i' : c.toLowerCase();
	return key;
}
