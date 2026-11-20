import { type AppMenuInput, appMenu } from './app-menu';
import { PANES } from './panes';

const input = (over: Partial<AppMenuInput> = {}): AppMenuInput => ({
  locked: false,
  busy: false,
  modules: ['git', 'calendar'],
  panes: PANES.slice(0, 3),
  isCurrentMonth: false,
  t: (k) => k,
  run: vi.fn(),
  go: vi.fn(),
  shiftMonth: vi.fn(),
  thisMonth: vi.fn(),
  openExport: vi.fn(),
  openGuide: vi.fn(),
  openModules: vi.fn(),
  ...over,
});
const file = (m: ReturnType<typeof appMenu>) => m.find((i) => i.label === 'menu.file')?.submenu ?? [];

describe('appMenu', () => {
  it('has only the system menus while locked', () => {
    expect(appMenu(input({ locked: true })).map((i) => i.role)).toEqual(['appMenu', 'editMenu', 'windowMenu']);
  });

  it('lists a fetch only for a switched-on module, and disables jobs while one runs', () => {
    const labels = (m: AppMenuInput['modules']) => file(appMenu(input({ modules: m }))).map((i) => i.label);
    expect(labels(['git'])).not.toContain('toolbar.fetchChat');
    expect(labels(['git', 'chats', 'workTimes'])).toEqual(
      expect.arrayContaining(['toolbar.fetchChat', 'toolbar.fetchHrmaster']),
    );
    const collect = file(appMenu(input({ busy: true }))).find((i) => i.label === 'toolbar.collectRange');
    expect(collect?.enabled).toBe(false);
  });

  it('runs what an item names', () => {
    const m = input();
    file(appMenu(m))
      .find((i) => i.label === 'toolbar.collectRange')
      ?.action?.();
    expect(m.run).toHaveBeenCalledWith('collect');
  });
});
