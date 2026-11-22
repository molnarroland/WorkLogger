import { Events } from '@wailsio/runtime';
import * as Go from '@bindings/github.com/molnarroland/WorkLogger/app/service';
import type {
  JiraAccount as GoJiraAccount,
  Profile as GoProfile,
  Settings as GoSettings,
} from '@bindings/github.com/molnarroland/WorkLogger/internal/core/models';
import type { RepoDTO, WorklogDTO as GoWorklog } from '@bindings/github.com/molnarroland/WorkLogger/app/models';
import type {
  ActivityLine,
  Api,
  ChatsDto,
  ChatMessageDto,
  CheckLine,
  IssueOptionDto,
  ExportFormat,
  JiraAccount,
  MeetingsDto,
  MonthDto,
  JiraActivityDto,
  ConfluenceActivityDto,
  ConfluenceChangeDto,
  EmailsDto,
  PlansDto,
  Profile,
  ProfileEdit,
  ProfileKind,
  ProjectDto,
  ProjectPeriod,
  RepoDto,
  SessionDayDto,
  SettingsDto,
  SetupStepDto,
  Status,
  TokenKind,
  WebSource,
  WeekDto,
  WorklogDto,
} from './api';

// The Go DTOs carry the same JSON as the hand-written ones in api.ts; the generated types only differ in
// string unions vs plain strings, hence the casts. Go never sends null for a list, but the bindings say it may.
const list = <T>(v: unknown): T[] => ((v as T[] | null) ?? []) as T[];

/** The real Api: each method is the generated Wails binding of app.Service (Go). The only Wails import. */
export class WailsApi implements Api {
  unlock = () => Go.Unlock();
  isLocked = () => Go.IsLocked();
  settings = async () => (await Go.Settings()) as unknown as SettingsDto;
  setupSteps = async () => list<SetupStepDto>(await Go.SetupSteps());
  month = async (month: string) => (await Go.Month(month)) as unknown as MonthDto;
  week = async (day: string) => (await Go.Week(day)) as unknown as WeekDto;
  refreshTempo = (month: string) => Go.RefreshTempo(month);
  plans = async (month: string) => (await Go.Plans(month)) as unknown as PlansDto;
  jiraActivity = async (from: string, to: string) => (await Go.JiraActivity(from, to)) as unknown as JiraActivityDto;
  sourceCounts = (from: string, to: string) => Go.SourceCounts(from, to);
  openJiraIssue = (account: string, key: string) => Go.OpenJiraIssue(account, key);
  issueOptions = async (day: string) => ((await Go.IssueOptions(day)) ?? []) as IssueOptionDto[];
  searchJiraIssues = async (day: string, text: string, weeks: number) =>
    ((await Go.SearchJiraIssues(day, text, weeks)) ?? []) as IssueOptionDto[];
  worklogCommits = async (id: number) => (await Go.WorklogCommits(id)) ?? [];
  commitFiles = async (repoId: number, hash: string) => (await Go.CommitFiles(repoId, hash)) ?? [];
  commitDiff = (repoId: number, hash: string, path: string) => Go.CommitDiff(repoId, hash, path);
  confluenceActivity = async (from: string, to: string) =>
    (await Go.ConfluenceActivity(from, to)) as unknown as ConfluenceActivityDto;
  openConfluencePage = (profile: string, pageId: string) => Go.OpenConfluencePage(profile, pageId);
  confluenceChanges = async (profile: string, kind: string, pageId: string, day: string) =>
    list<ConfluenceChangeDto>(await Go.ConfluenceChanges(profile, kind, pageId, day));

  updateWorklog = async (w: WorklogDto) => (await Go.UpdateWorklog(w as unknown as GoWorklog)) as unknown as WorklogDto;
  setDuration = async (id: number, text: string) => (await Go.SetDuration(id, text)) as unknown as WorklogDto;
  moveWorklogs = (ids: number[], toDay: string) => Go.MoveWorklogs(ids, toDay);
  addWorklog = async (day: string) => (await Go.AddWorklog(day)) as unknown as WorklogDto;
  deleteWorklog = (id: number) => Go.DeleteWorklog(id);
  deleteWorklogs = (ids: number[]) => Go.DeleteWorklogs(ids);
  setStatus = (ids: number[], status: Status) => Go.SetStatus(ids, status);
  setIssue = (ids: number[], issueKey: string) => Go.SetIssue(ids, issueKey);
  setEntered = (ids: number[], entered: boolean) => Go.SetEntered(ids, entered);
  setComment = (ids: number[], comment: string) => Go.SetComment(ids, comment);
  useDescriptionsAsComments = (day: string) => Go.UseDescriptionsAsComments(day);

  saveSettings = async (s: SettingsDto) =>
    (await Go.SaveSettings(s as unknown as GoSettings)) as unknown as SettingsDto;
  addPeriod = async (month: string) => this.settingsOf(Go.AddPeriod(month));
  updatePeriod = async (p: ProjectPeriod) => this.settingsOf(Go.UpdatePeriod(p));
  removePeriod = async (id: string) => this.settingsOf(Go.RemovePeriod(id));
  addMeetingRule = async (contains: string, issueKey: string) => this.settingsOf(Go.AddMeetingRule(contains, issueKey));
  removeMeetingRule = async (id: string) => this.settingsOf(Go.RemoveMeetingRule(id));
  addAbsenceRule = async (contains: string, issueKey: string) => this.settingsOf(Go.AddAbsenceRule(contains, issueKey));
  removeAbsenceRule = async (id: string) => this.settingsOf(Go.RemoveAbsenceRule(id));
  setChatRule = async (conversationId: string, name: string, issueKey: string) =>
    this.settingsOf(Go.SetChatRule(conversationId, name, issueKey));
  addWorkspaceRoot = async (path: string) => this.settingsOf(Go.AddWorkspaceRoot(path));
  removeWorkspaceRoot = async (path: string) => this.settingsOf(Go.RemoveWorkspaceRoot(path));
  private async settingsOf(p: Promise<GoSettings>) {
    return (await p) as unknown as SettingsDto;
  }
  databasePath = () => Go.DatabasePath();
  chooseFolder = () => Go.ChooseFolder();
  revealPath = (path: string) => Go.RevealPath(path);

  repos = async (month: string) => list<RepoDto>(await Go.Repos(month));
  updateRepo = (r: RepoDto) => Go.UpdateRepo(r as unknown as RepoDTO);
  removeRepo = (id: number) => Go.RemoveRepo(id);
  scanWorkspace = () => Go.ScanWorkspace();
  projects = async () => list<ProjectDto>(await Go.Projects());
  saveProject = (p: ProjectDto) => Go.SaveProject(p);
  removeProject = (key: string) => Go.RemoveProject(key);

  sessions = async (month: string) => {
    const r = await Go.Sessions(month);
    return { days: list<SessionDayDto>(r.days) };
  };
  clearSessions = (from: string, to: string) => Go.ClearSessions(from, to);

  meetings = async (from: string, to: string) => (await Go.Meetings(from, to)) as unknown as MeetingsDto;
  setMeetingSkipped = (id: string, skipped: boolean) => Go.SetMeetingSkipped(id, skipped);
  calendarStatus = () => Go.CalendarStatus();
  connectGoogle = () => Go.ConnectGoogle();
  disconnectGoogle = () => Go.DisconnectGoogle();

  chats = async (from: string, to: string) => (await Go.Chats(from, to)) as unknown as ChatsDto;
  chatMessages = async (id: string) => list<ChatMessageDto>(await Go.ChatMessages(id));
  logChat = (id: string, issueKey: string) => Go.LogChat(id, issueKey);
  clearChats = (from: string, to: string) => Go.ClearChats(from, to);
  importChats = (text: string) => Go.ImportChats(text);

  signedIn = (source: WebSource) => Go.SignedIn(source);
  openSite = (source: WebSource) => Go.OpenSite(source);
  signOut = (source: WebSource) => Go.SignOut(source);

  hasToken = (accountId: string, kind: TokenKind) => Go.HasToken(accountId, kind);
  setToken = (accountId: string, kind: TokenKind, value: string) => Go.SetToken(accountId, kind, value);
  hasGoogleClientSecret = () => Go.HasGoogleClientSecret();
  setGoogleClientSecret = (value: string) => Go.SetGoogleClientSecret(value);
  addProfile = async (kind: ProfileKind) => (await Go.AddProfile(kind)) as unknown as ProfileEdit;
  removeProfile = async (id: string) => this.settingsOf(Go.RemoveProfile(id));
  updateProfile = async (p: Profile) => this.settingsOf(Go.UpdateProfile(p as unknown as GoProfile));
  updateJiraAccount = async (a: JiraAccount) => this.settingsOf(Go.UpdateJiraAccount(a as unknown as GoJiraAccount));
  setDefaultJiraAccount = async (id: string) => this.settingsOf(Go.SetDefaultJiraAccount(id));
  checkJira = async (accountId: string) => list<CheckLine>(await Go.CheckJira(accountId));
  checkTempo = async (accountId: string) => list<CheckLine>(await Go.CheckTempo(accountId));
  checkEmail = async (profileId: string) => list<CheckLine>(await Go.CheckEmail(profileId));
  sentEmails = async (from: string, to: string) => (await Go.SentEmails(from, to)) as unknown as EmailsDto;
  checkConfluence = async (profileId: string) => list<CheckLine>(await Go.CheckConfluence(profileId));
  readProjects = async (accountId: string) => list<string>(await Go.ReadProjects(accountId));

  exportText = (ids: number[]) => Go.ExportText(ids);
  exportFile = (ids: number[], format: ExportFormat) => Go.ExportFile(ids, format);
  pushDayToTempo = (day: string) => Go.PushDayToTempo(day);
  pushPreview = async (from: string, to: string) => {
    const p = await Go.PushPreview(from, to);
    return { ...p, draftDays: p.draftDays ?? [] };
  };
  pushRangeToTempo = (from: string, to: string) => Go.PushRangeToTempo(from, to);

  collect = (from: string, to: string, force = false) => Go.Collect(from, to, force);
  cleanUpSettled = (from: string, to: string) => Go.CleanUpSettled(from, to);
  rebuild = (from: string, to: string) => Go.Rebuild(from, to);
  fetchHrmaster = (from: string, to: string) => Go.FetchHrmaster(from, to);
  fetchGoogleChat = (from: string, to: string) => Go.FetchGoogleChat(from, to);
  fetchGoogleCalendar = (from: string, to: string) => Go.FetchGoogleCalendar(from, to);
  cancel = () => Go.Cancel();

  activity = async () => list<ActivityLine>(await Go.Activity());
  onEvent<T>(name: string, cb: (data: T) => void): () => void {
    return Events.On(name, (ev) => cb(ev.data as T));
  }
}
