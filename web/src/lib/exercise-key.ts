/** Leading and trailing white space: JS \s plus U+0085, which Go also trims. */
const EDGE_SPACE = /^[\s\u0085]+|[\s\u0085]+$/g;

export function trimName(name: string): string {
	return name.replace(EDGE_SPACE, '');
}
