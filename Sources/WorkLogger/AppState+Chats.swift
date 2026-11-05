import Foundation
import SwiftUI
import UniformTypeIdentifiers
import EventKit
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// Chats: Google Chat fetching, pasted exports, logging a conversation to a ticket.
extension AppState {
    // MARK: Chats

    /// Imports a pasted chat export; returns false (with the reason in the status) when it can't be read.
    func importChats(_ pasted: String) -> Bool {
        do {
            let export = try ChatExport.parse(pasted)
            let added = try db.importChats(export)
            reloadRange()
            status = "Imported \(added) new messages from \(export.conversations.count) conversations."
            return true
        } catch {
            status = "Chat import failed: \(error.localizedDescription)"
            return false
        }
    }

    func chatLines(_ c: ChatDay) -> [ChatLine] {
        (try? db.chatLines(conversationId: c.conversationId, day: c.day)) ?? []
    }

    /// The worklog a conversation-day was logged to, if any.
    func chatWorklog(_ c: ChatDay) -> Worklog? { worklogs.first { $0.source == .chat && $0.sourceRef == c.ref } }

    /// Logs a conversation-day's active time to an issue (or re-targets the existing entry); locked so rebuilds keep it.
    func logChat(_ c: ChatDay, issue: String) {
        guard let key = JiraKey.normalize(issue), c.activeSeconds > ChatDay.minLoggable else { return }
        saveDirty()
        let minutes = (c.activeSeconds + 59) / 60
        attempt("Logging chat") {
            if var w = chatWorklog(c) {
                guard w.status != .exported else { return }
                w.issueKey = key
                try db.updateWorklog(w)
            } else {
                try db.insertWorklog(Worklog(id: 0, day: c.day, start: c.times[0], duration: minutes * 60, issueKey: key,
                                             description: "Chat: \(c.name)", source: .chat, sourceRef: c.ref, repoId: nil,
                                             estimated: c.activeSeconds, status: .draft, locked: true, tempoId: nil))
            }
            status = "Logged \(DurationFormat.string(minutes * 60)) of “\(c.name)” to \(key)."
        }
        reloadRange()
    }

    /// Reads your Google Chat messages for the range in the background (the signed-in Google Chat page, no window)
    /// and imports them, with progress in the status bar. Returns true when you need to sign in first.
    @discardableResult
    func fetchGoogleChat() async -> Bool {
        guard !isBusy else { return false }
        let email = settings.chatEmail.isEmpty ? (settings.authorEmails.first ?? "") : settings.chatEmail
        guard !email.isEmpty else { status = "Enter your Google Chat email on the Chats page first."; return false }
        isBusy = true
        defer { isBusy = false }
        do {
            let export = try await GoogleChatCollector(session: WebSession.googleChat, email: email)
                .collect(from: rangeStart, to: rangeEnd) { self.status = $0 }
            let added = try db.importChats(export)
            reloadRange()
            status = "Google Chat: \(added) new messages in \(export.conversations.count) conversations."
        } catch WebSessionError.notSignedIn {
            status = "Google Chat is signed out. Sign in in the Google Chat window; fetching continues after."
            signInNeeded = .googleChat
            return true
        } catch {
            status = "Google Chat: \(error.localizedDescription)"
        }
        return false
    }

    func clearChatsInRange() {
        attempt("Clearing chats") { try db.deleteChats(from: rangeStart, to: rangeEnd) }
        reloadRange()
    }
}
