import { describe, expect, it } from 'vitest';
import { setupLink, toSetupSteps } from './setup-steps';

// Which steps exist and which are done is decided in Go (internal/view SetupSteps).
describe('setup steps', () => {
  it("sends a source's steps to its Setup tab, accounts and Tempo to Settings", () => {
    expect(
      toSetupSteps([
        { id: 'hrmaster', done: false, profile: 'hrmaster', page: '', tab: '' },
        { id: 'calendar', done: true, profile: '', page: 'settings', tab: 'calendars' },
        { id: 'jira', done: false, profile: 'jira', page: '', tab: '' },
        { id: 'tempo', done: false, profile: 'tempo', page: '', tab: '' },
      ]),
    ).toEqual([
      { id: 'hrmaster', done: false, action: { kind: 'page', path: 'sessions', tab: 'setup' } },
      { id: 'calendar', done: true, action: { kind: 'page', path: 'meetings', tab: 'setup' } },
      { id: 'jira', done: false, action: { kind: 'profile', profile: 'jira' } },
      { id: 'tempo', done: false, action: { kind: 'profile', profile: 'tempo' } },
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
