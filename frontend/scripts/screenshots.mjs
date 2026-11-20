// Screenshots of the app for the project page (site/img) and the README, taken from the built frontend in a plain
// browser, where the in-memory FakeApi serves the invented sample month (src/app/api/testing). No real data is read.
// Run it with `task screenshots` from the repository root (it builds the frontend first).
import { createReadStream, existsSync, statSync } from 'node:fs';
import { createServer } from 'node:http';
import { extname, join, resolve } from 'node:path';
import { chromium } from 'playwright';

const root = resolve(import.meta.dirname, '..');
const dist = join(root, 'dist');
const out = resolve(root, '../site/img');

// Each shot: file name, route (the sample's "today" is 2026-10-30), and colour scheme.
const shots = [
  { file: 'overview.jpg', route: 'overview' },
  { file: 'overview-dark.jpg', route: 'overview', scheme: 'dark' },
  { file: 'days.jpg', route: 'days/2026-10-27' },
  { file: 'week.jpg', route: 'week/2026-10-27' },
  { file: 'timesheet.jpg', route: 'timesheet' },
  { file: 'by-ticket.jpg', route: 'by-ticket' },
];

const types = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.json': 'application/json',
  '.svg': 'image/svg+xml',
  '.png': 'image/png',
  '.ico': 'image/x-icon',
  '.woff2': 'font/woff2',
};

if (!existsSync(join(dist, 'index.html'))) {
  console.error('No frontend/dist: build the frontend first (task screenshots does).');
  process.exit(1);
}

const server = createServer((req, res) => {
  const path = join(dist, decodeURIComponent(new URL(req.url, 'http://x').pathname));
  const file = path.startsWith(dist) && existsSync(path) && statSync(path).isFile() ? path : join(dist, 'index.html');
  res.writeHead(200, { 'content-type': types[extname(file)] ?? 'application/octet-stream' });
  createReadStream(file).pipe(res);
});
await new Promise((ok) => server.listen(0, '127.0.0.1', ok));
const base = `http://127.0.0.1:${server.address().port}/`;

const browser = await chromium.launch();
try {
  for (const s of shots) {
    const context = await browser.newContext({
      // 1600×1000 images, like the page's <img width/height>.
      viewport: { width: 1280, height: 800 },
      deviceScaleFactor: 1.25,
      colorScheme: s.scheme ?? 'light',
      reducedMotion: 'reduce',
      locale: 'en-US',
      timezoneId: 'Europe/Berlin', // the sample's times are written for UTC+1
    });
    const page = await context.newPage();
    page.on('pageerror', (e) => console.error(`${s.file}: ${e.message}`));
    await page.goto(`${base}#/${s.route}`, { waitUntil: 'networkidle' });
    await page.evaluate(() => document.fonts.ready);
    await page.waitForTimeout(500); // the FakeApi answers on a timer; let the last view settle
    await page.screenshot({ path: join(out, s.file), type: 'jpeg', quality: 85 });
    console.log(`site/img/${s.file}`);
    await context.close();
  }
} finally {
  await browser.close();
  server.close();
}
