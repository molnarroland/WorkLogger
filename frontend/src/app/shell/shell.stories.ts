import type { Meta, StoryObj } from '@storybook/angular-vite';
import { expect, screen, waitFor } from 'storybook/test';
import { fixtures, freshSettings } from '../api/testing/fixtures';
import { ShellComponent } from './shell.component';

const meta: Meta<ShellComponent> = {
  title: 'Pages/Shell',
  component: ShellComponent,
  parameters: { ownMain: true },
};
export default meta;
type Story = StoryObj<ShellComponent>;

/** The status line as screen readers hear it (the visible message shows only while there is one). */
const announced = (root: HTMLElement) => root.querySelector<HTMLElement>('[role=status].sr-only');

export const Unlocked: Story = {};
export const Locked: Story = { parameters: { api: { locked: true } } };
export const GitOnly: Story = { parameters: { api: { settings: fixtures.gitOnly } } };
export const Hungarian: Story = {
  parameters: { lang: 'hu' },
  // The settings' language applies once they load; the story's stays (the fake's settings carry it).
  play: async ({ canvas }) => {
    await expect(await canvas.findByRole('button', { name: /^Begyűjtés/ })).toBeVisible();
  },
};
/** axe measures text against white here: it can't see through the macOS 26 materials (backdrop blur), so its
 *  contrast check is off for this story only; every other rule still fails the run. Light mode and the other
 *  skins are checked for contrast too. */
export const MacOSDark: Story = {
  globals: { skin: 'mac', scheme: 'dark' },
  parameters: { a11y: { config: { rules: [{ id: 'color-contrast', enabled: false }] } } },
};
export const Gnome: Story = { globals: { skin: 'gtk' } };

export const UnlockThenNavigate: Story = {
  parameters: { api: { locked: true } },
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'Unlock' }));
    await userEvent.click(await canvas.findByRole('link', { name: /^Days/ }));
    await expect(await canvas.findByRole('heading', { name: 'Days' }, { timeout: 5000 })).toBeVisible();
  },
};

export const CollectShowsProgress: Story = {
  play: async ({ canvas, canvasElement, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: /^Collect/ }));
    await waitFor(() => expect(announced(canvasElement)).toHaveTextContent(fixtures.collectSteps[0].message.text));
    await waitFor(() => expect(canvas.queryByRole('button', { name: 'Cancel' })).toBeNull(), { timeout: 3000 });
  },
};

export const MonthShortcuts: Story = {
  play: async ({ canvas, userEvent }) => {
    await canvas.findAllByText('October 2026');
    await userEvent.keyboard('{Control>}[BracketRight]{/Control}');
    await expect((await canvas.findAllByText('November 2026'))[0]).toBeVisible();
  },
};

export const FirstRun: Story = {
  parameters: { api: { settings: freshSettings() } },
  play: async ({ userEvent }) => {
    const dialog = await screen.findByRole('dialog', { name: 'Welcome to WorkLogger' });
    await expect(dialog).toBeVisible();
    await userEvent.click(await screen.findByRole('switch', { name: 'Do you commit to Git repositories?' }));
    await userEvent.click(screen.getByRole('button', { name: 'Next' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Add folder…' }));
    await userEvent.click(await screen.findByRole('button', { name: 'Done' }));
    await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Welcome to WorkLogger' })).toBeNull(), {
      timeout: 3000,
    });
  },
};

export const FoldTheSidePanel: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'Collapse the side panel' }));
    // Folded: icons only, but every link keeps its name.
    await expect(await canvas.findByRole('button', { name: 'Expand the side panel' })).toHaveAttribute(
      'aria-expanded',
      'false',
    );
    await userEvent.click(canvas.getByRole('link', { name: /^Week/ }));
    await expect(await canvas.findByRole('heading', { name: 'Week' }, { timeout: 5000 })).toBeVisible();
    await userEvent.click(canvas.getByRole('button', { name: 'Expand the side panel' }));
  },
};

export const OpenAPageFromAllPages: Story = {
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'All pages' }));
    await userEvent.click(await canvas.findByRole('button', { name: 'Timesheet' }));
    await expect(await canvas.findByRole('heading', { name: 'Timesheet' }, { timeout: 5000 })).toBeVisible();
    // The item now stands for the open page.
    await expect(canvas.getByRole('button', { name: 'Timesheet' })).toHaveAttribute('aria-current', 'page');
  },
};

/**
 * The smoke test of the main flow, against the fake API (RELEASE_REVIEW D9): unlock, collect, review, send to
 * Tempo. The Go side of each step has its own tests; this one checks the screens are wired together.
 */
export const SmokeUnlockCollectReviewSend: Story = {
  parameters: { api: { locked: true } },
  // Start on the default page: the hash URL outlives a story, and "All pages" is named after the page it opened.
  beforeEach: () => history.replaceState(null, '', location.pathname + location.search),
  play: async ({ canvas, canvasElement, userEvent }) => {
    await userEvent.click(await canvas.findByRole('button', { name: 'Unlock' }));

    await userEvent.click(await canvas.findByRole('button', { name: /^Collect/ }));
    await waitFor(() => expect(canvas.queryByRole('button', { name: 'Cancel' })).toBeNull(), { timeout: 5000 });

    await userEvent.click(canvas.getByRole('button', { name: 'All pages' }));
    await userEvent.click(await canvas.findByRole('button', { name: 'Review' }));
    await expect(await canvas.findByRole('heading', { name: 'Review' }, { timeout: 5000 })).toBeVisible();
    await userEvent.click(await canvas.findByRole('button', { name: 'All' }));
    await expect((await canvas.findAllByRole('row')).length).toBeGreaterThan(1);

    // Send one day by hand: the 26th, its drafts reviewed first (they need their descriptions as comments).
    await userEvent.click(canvas.getByRole('link', { name: /^Days/ }));
    const day = () => canvas.findByRole('heading', { level: 2, name: /October \d+/ }, { timeout: 5000 });
    const goTo26 = async () => {
      for (let i = 0; i < 5 && !/October 26/.test((await day()).textContent ?? ''); i++)
        await userEvent.click(canvas.getByRole('button', { name: 'Previous day' }));
      await expect(await day()).toHaveTextContent(/October 26/);
    };
    await goTo26();
    await userEvent.click(await canvas.findByRole('button', { name: 'Use the descriptions' }));
    await waitFor(() => expect(canvas.getByRole('button', { name: 'Mark reviewed and go to next' })).toBeEnabled());
    await userEvent.click(canvas.getByRole('button', { name: 'Mark reviewed and go to next' }));
    await goTo26();
    const send = await canvas.findByRole('button', { name: 'Send day to Tempo' });
    await waitFor(() => expect(send).toBeEnabled());
    await userEvent.click(send);
    await userEvent.click(await screen.findByRole('button', { name: 'Send to Tempo' }));
    await waitFor(() => expect(announced(canvasElement)).toHaveTextContent(/^Sent \d+ worklogs? for .+ to Tempo/), {
      timeout: 5000,
    });
  },
};

/** macOS joins status items with commas; with no message the line starts at "1 needs a ticket", not ", 1 needs…". */
export const MacOSStatusLine: Story = {
  globals: { skin: 'mac' },
  parameters: { a11y: { config: { rules: [{ id: 'color-contrast', enabled: false }] } } },
  play: async ({ canvasElement }) => {
    await waitFor(() => expect(canvasElement.querySelector('sn-status-bar .needs-ticket')).toBeTruthy());
    const first = canvasElement.querySelector('sn-status-bar [data-slot=sn-status-item]')!;
    await expect(first.textContent?.trim()).toMatch(/^1 needs a ticket$/);
    await expect(getComputedStyle(first, '::before').content).toBe('none');
  },
};

/** An entry's "Show in …" opens its source: here the standup, on the Meetings page narrowed to its day. */
export const ShowAnEntrysSource: Story = {
  beforeEach: () => history.replaceState(null, '', location.pathname + location.search),
  play: async ({ canvas, userEvent }) => {
    await userEvent.click(await canvas.findByRole('link', { name: /^Days/ }));
    await userEvent.click(await canvas.findByText('Daily standup', {}, { timeout: 5000 }));
    await userEvent.click(await canvas.findByRole('button', { name: 'Show in Meetings' }));
    await expect(await canvas.findByRole('heading', { name: 'Meetings' }, { timeout: 5000 })).toBeVisible();
    await expect(await canvas.findByText(/^Only /)).toBeVisible();
    const row = (await canvas.findByText('Daily standup')).closest('tr')!;
    await expect(row).toHaveAttribute('aria-current', 'true');
  },
};
