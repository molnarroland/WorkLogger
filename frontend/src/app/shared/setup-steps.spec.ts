import { describe, expect, it } from 'vitest';
import { setupLink, toSetupSteps } from './setup-steps';

// Which steps exist and which are done is decided in Go (internal/view SetupSteps).
describe('setup steps', () => {
  it('gives each backend step where it is done', () => {
    expect(
      toSetupSteps([
        { id: 'hrmaster', done: false, profile: 'hrmaster', page: '', tab: '' },
        { id: 'calendar', done: true, profile: '', page: 'settings', tab: 'calendars' },
      ]),
    ).toEqual([
      { id: 'hrmaster', done: false, action: { kind: 'profile', profile: 'hrmaster' } },
      {
        id: 'calendar',
        done: true,
        action: { kind: 'page', path: 'settings', tab: 'calendars' },
      },
    ]);
  });

  it('links profiles to Settings → Profiles with the kind to add', () => {
    expect(setupLink({ kind: 'profile', profile: 'hrmaster' })).toEqual({
      commands: ['settings'],
      queryParams: { tab: 'profiles', add: 'hrmaster' },
    });
    expect(setupLink({ kind: 'page', path: 'repositories' })).toEqual({ commands: ['repositories'], queryParams: {} });
  });
});
