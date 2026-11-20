import { existsSync, readdirSync } from 'node:fs';
import { join } from 'node:path';

// Every component in ui/, pages/ and shell/ needs a story (CONTRIBUTING.md).
describe('stories', () => {
  const root = join(process.cwd(), 'src/app');
  const components = ['ui', 'pages', 'shell'].flatMap((dir) =>
    readdirSync(join(root, dir), { recursive: true, encoding: 'utf8' })
      .filter((f) => f.endsWith('.component.ts'))
      .map((f) => join(root, dir, f)),
  );

  it('finds components', () => expect(components.length).toBeGreaterThan(0));

  it.each(components)('%s has a .stories.ts', (file) => {
    expect(existsSync(file.replace(/\.component\.ts$/, '.stories.ts'))).toBe(true);
  });
});
