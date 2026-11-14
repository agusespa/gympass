// Generates the app icons from one SVG design: npm run icons
// Writes the favicon SVG and the PNGs in web/static (rendered with Playwright's Chromium).
import { writeFileSync } from 'node:fs';
import { chromium } from '@playwright/test';

const FROM = '#ff9a4d';
const TO = '#c2410c';

// White dumbbell on a 512×512 canvas: bar, collars and two plates a side, drawn
// for the left half and mirrored. Tilted and scaled so its farthest point stays
// inside the maskable safe zone (a circle of radius 204 around the centre).
const HALF = `<rect x="96" y="234" width="22" height="44" rx="7"/>
    <rect x="114" y="178" width="40" height="156" rx="14"/>
    <rect x="160" y="146" width="48" height="220" rx="16"/>
    <rect x="208" y="226" width="14" height="60" rx="5"/>
    <rect x="131" y="196" width="6" height="120" rx="3" fill="#fbd5b8"/>
    <rect x="181" y="166" width="6" height="180" rx="3" fill="#fbd5b8"/>`;

const GLYPH = `<g transform="rotate(-30 256 256) translate(256 256) scale(1.12) translate(-256 -256)" fill="#fff" filter="url(#shadow)">
  <rect x="200" y="241" width="112" height="30" rx="7"/>
  <g>${HALF}</g>
  <g transform="translate(512 0) scale(-1 1)">${HALF}</g>
</g>`;

/** rounded: transparent corners (favicon, manifest "any"); otherwise a full square (iOS, maskable). */
function svg(rounded) {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<defs>
<linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${FROM}"/><stop offset="1" stop-color="${TO}"/></linearGradient>
<radialGradient id="glow" cx="0.3" cy="0.2" r="0.8"><stop offset="0" stop-color="#fff" stop-opacity=".22"/><stop offset="1" stop-color="#fff" stop-opacity="0"/></radialGradient>
<filter id="shadow" x="-20%" y="-20%" width="140%" height="140%"><feDropShadow dx="0" dy="10" stdDeviation="12" flood-color="#7c2d12" flood-opacity=".45"/></filter>
</defs>
<rect width="512" height="512"${rounded ? ' rx="112"' : ''} fill="url(#bg)"/>
<rect width="512" height="512"${rounded ? ' rx="112"' : ''} fill="url(#glow)"/>
${GLYPH}
</svg>
`;
}

const outputs = [
	{ file: 'web/static/icon-192.png', size: 192, rounded: true },
	{ file: 'web/static/icon-512.png', size: 512, rounded: true },
	{ file: 'web/static/icon-maskable-512.png', size: 512, rounded: false },
	{ file: 'web/static/apple-touch-icon.png', size: 180, rounded: false }
];

writeFileSync('web/src/lib/assets/favicon.svg', svg(true));

const browser = await chromium.launch();
try {
	for (const { file, size, rounded } of outputs) {
		const page = await browser.newPage({ viewport: { width: size, height: size } });
		const img = svg(rounded).replace('<svg ', `<svg width="${size}" height="${size}" `);
		await page.setContent(`<style>html,body{margin:0;background:transparent}svg{display:block}</style>${img}`);
		await page.screenshot({ path: file, omitBackground: rounded });
		await page.close();
		console.log(`wrote ${file}`);
	}
} finally {
	await browser.close();
}
