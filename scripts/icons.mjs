// Generates the app icons from one SVG design: npm run icons
// Writes the favicon SVG and the PNGs in web/static (rendered with Playwright's Chromium).
import { writeFileSync } from 'node:fs';
import { chromium } from '@playwright/test';

const FROM = '#27272a';
const TO = '#09090b';
const INK = '#bef264';
const LIGHT = '#d9f99d';

// A chunky lime lightning bolt with slightly rough edges and a
// lighter left facet, on a 512×512 canvas. Every point stays inside the
// maskable safe zone (a circle of radius 204 around the centre).
const BOLT = '232,68 326,68 282,212 360,212 226,450 256,298 160,298';
// The left half of the bolt, lit a shade lighter for a bevelled look.
const FACET = '232,68 279,68 236,212 282,212 226,450 256,298 160,298';

const DEFS = `<linearGradient id="bg" x1="0" y1="0" x2="1" y2="1"><stop offset="0" stop-color="${FROM}"/><stop offset="1" stop-color="${TO}"/></linearGradient>
<filter id="rough" x="-10%" y="-10%" width="120%" height="120%">
  <feTurbulence type="fractalNoise" baseFrequency="0.05" numOctaves="2" seed="7" result="warp"/>
  <feDisplacementMap in="SourceGraphic" in2="warp" scale="3" xChannelSelector="R" yChannelSelector="G"/>
</filter>`;

/** rounded: transparent corners (favicon, manifest "any"); otherwise a full square (iOS, maskable). */
function svg(rounded) {
	return `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512">
<defs>${DEFS}</defs>
<rect width="512" height="512"${rounded ? ' rx="112"' : ''} fill="url(#bg)"/>
<g filter="url(#rough)" stroke-linejoin="round">
  <polygon points="${BOLT}" fill="${INK}"/>
  <polygon points="${FACET}" fill="${LIGHT}"/>
</g>
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
