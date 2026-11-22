/**
 * Compile-time check that the hand-written DTOs in api.ts have exactly the fields of the Go DTOs, as Wails
 * generates them in bindings/ (RELEASE_REVIEW C6). api.ts keeps its own types for the string unions the generated
 * ones lack (`Status` instead of `string`), and wails-api.ts casts between the two; this file is what makes that
 * cast safe. When a Go DTO gains, loses or renames a field, `npm run build` fails here until api.ts follows.
 * Type-only: nothing in it reaches the bundle.
 */
import type * as App from '@bindings/github.com/molnarroland/WorkLogger/app/models';
import type * as Core from '@bindings/github.com/molnarroland/WorkLogger/internal/core/models';
import type * as View from '@bindings/github.com/molnarroland/WorkLogger/internal/view/models';
import type * as Api from './api';

type Keys<T> = Extract<keyof T, string>;
/** true, or which fields differ, so the error names them. */
type Same<Ts, Go, GoOnlyAllowed extends string = never> = [
  Exclude<Keys<Go>, Keys<Ts> | GoOnlyAllowed>,
  Exclude<Keys<Ts>, Keys<Go>>,
] extends [never, never]
  ? true
  : { onlyInGo: Exclude<Keys<Go>, Keys<Ts> | GoOnlyAllowed>; onlyInTs: Exclude<Keys<Ts>, Keys<Go>> };
type Assert<T extends true> = T;

export type BindingsMatch = [
  Assert<Same<Api.ActivityLine, App.ActivityLine>>,
  Assert<Same<Api.BackendMessage, Core.Message>>,
  Assert<Same<Api.CalendarStatus, App.CalendarStatus>>,
  Assert<Same<Api.ChatDayDto, App.ChatDayDTO>>,
  Assert<Same<Api.CommitDto, App.CommitDTO>>,
  Assert<Same<Api.CommitFileDto, App.CommitFileDTO>>,
  Assert<Same<Api.CommitDiffDto, App.CommitDiffDTO>>,
  Assert<Same<Api.IssueOptionDto, App.IssueOptionDTO>>,
  Assert<Same<Api.PushPreviewDto, App.PushPreviewDTO>>,
  Assert<Same<Api.ChatMessageDto, App.ChatMessageDTO>>,
  Assert<Same<Api.ChatsDto, App.ChatsDTO>>,
  Assert<Same<Api.ChatsDto['days'][number], App.ChatGroupDTO>>,
  Assert<Same<Api.CollectDone, App.CollectDone>>,
  Assert<Same<Api.CleanupDto, App.CleanupDTO>>,
  Assert<Same<Api.CollectProgress, App.CollectProgress>>,
  Assert<Same<Api.DayDto, App.DayDTO>>,
  Assert<Same<Api.ExportChoiceDto, App.ExportChoiceDTO>>,
  Assert<Same<Api.MeetingDto, App.MeetingDTO>>,
  Assert<Same<Api.MeetingsDto, App.MeetingsDTO>>,
  Assert<Same<Api.MonthDto, App.MonthDTO>>,
  Assert<Same<Api.PartDto, App.PartDTO>>,
  Assert<Same<Api.JiraActivityDto, App.JiraActivityDTO>>,
  Assert<Same<Api.SourceCountsDto, App.SourceCountsDTO>>,
  Assert<Same<Api.JiraChangeDto, App.JiraChangeDTO>>,
  Assert<Same<Api.JiraDayDto, App.JiraDayDTO>>,
  Assert<Same<Api.JiraNoteDto, App.JiraNoteDTO>>,
  Assert<Same<Api.JiraTicketDto, App.JiraTicketDTO>>,
  Assert<Same<Api.ConfluenceActivityDto, App.ConfluenceActivityDTO>>,
  Assert<Same<Api.ConfluenceDayDto, App.ConfluenceDayDTO>>,
  Assert<Same<Api.ConfluencePageDto, App.ConfluencePageDTO>>,
  Assert<Same<Api.ConfluenceChangeDto, App.ConfluenceChangeDTO>>,
  Assert<Same<Api.EmailsDto, App.EmailsDTO>>,
  Assert<Same<Api.EmailDayDto, App.EmailDayDTO>>,
  Assert<Same<Api.EmailThreadDto, App.EmailThreadDTO>>,
  Assert<Same<Api.PlansDto, App.PlansDTO>>,
  Assert<Same<Api.PlanDayDto, App.PlanDayDTO>>,
  Assert<Same<Api.PlanItemDto, App.PlanItemDTO>>,
  Assert<Same<Api.PlanEntryDto, App.PlanEntryDTO>>,
  Assert<Same<Api.ProfileEdit, App.ProfileEdit>>,
  Assert<Same<Api.ProjectDto, App.ProjectDTO>>,
  Assert<Same<Api.RepoDto, App.RepoDTO>>,
  Assert<Same<Api.ReportGroupDto, App.ReportGroupDTO>>,
  Assert<Same<Api.SessionDayDto, App.SessionDayDTO>>,
  Assert<Same<Api.SessionDto, App.SessionDTO>>,
  Assert<Same<Api.SetupStepDto, App.SetupStepDTO>>,
  Assert<Same<Api.SummaryDto, App.SummaryDTO>>,
  Assert<Same<Api.TicketDayDto, App.TicketDayDTO>>,
  Assert<Same<Api.TicketGroupDto, App.TicketGroupDTO>>,
  Assert<Same<Api.TodoDto, App.TodoDTO>>,
  Assert<Same<Api.WeekDto, App.WeekDTO>>,
  Assert<Same<Api.WorklogDto, App.WorklogDTO>>,
  Assert<Same<Api.Focus, View.Focus>>,
  // Settings: the backend keeps two keys the frontend never sees (an unused key kept for older databases, the known modules).
  Assert<Same<Api.SettingsDto, Core.Settings, 'knownModules' | 'macCalendarIds'>>,
  Assert<Same<Api.Profile, Core.Profile>>,
  Assert<Same<Api.JiraAccount, Core.JiraAccount>>,
  Assert<Same<Api.ProjectPeriod, Core.ProjectPeriod>>,
  Assert<Same<Api.MeetingRule, Core.MeetingRule>>,
  Assert<Same<Api.ChatRule, Core.ChatRule>>,
];
