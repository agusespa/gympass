// Generates the app icons from one SVG design: npm run icons
// Writes the favicon SVG and the PNGs in web/static (rendered with Playwright's Chromium).
import { writeFileSync } from 'node:fs';
import { chromium } from '@playwright/test';

const FROM = '#ff9a4d';
const TO = '#c2410c';

// White dumbbell on a 512×512 canvas, tilted. Rotation keeps it inside the
// maskable safe zone (a circle of radius 204 around the centre).
const GLYPH = `<g transform="rotate(-35 256 256)" stroke="#fff" stroke-linecap="round" fill="none">
  <line x1="186" y1="256" x2="326" y2="256" stroke-width="28"/>
  <line x1="128" y1="216" x2="128" y2="296" stroke-width="40"/>
  <line x1="186" y1="176" x2="186" y2="336" stroke-width="40"/>
  <line x1="326" y1="176" x2="326" y2="336" stroke-width="40"/>
  <line x1="384" y1="216" x2="384" y2="296" stroke-width="40"/>
</g>`;

/** rounded: transparent corners (favicon, manifest "any"); otherwise a full square (iOS, maskable). */
function svg(rounded) {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<defs><linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${FROM}"/><stop offset="1" stop-color="${TO}"/></linearGradient></defs>
<rect width="512" height="512"${rounded ? ' rx="112"' : ''} fill="url(#bg)"/>
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
