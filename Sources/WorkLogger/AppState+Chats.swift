import EventKit
import Foundation
import SwiftUI
import UniformTypeIdentifiers
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
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
            applyChatRules()
            status = String(localized: "Imported \(added) new messages from \(export.conversations.count) conversations.")
            return true
        } catch {
            status = String(localized: "Chat import failed: \(error.localizedDescription)")
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
        attempt("Logging chat") {
            try writeChatWorklog(c, key: key)
            status = String(localized: "Logged \(DurationFormat.string(Self.chatMinutes(c) * 60)) of “\(c.name)” to \(key).")
        }
        reloadRange()
    }

    private static func chatMinutes(_ c: ChatDay) -> Int { (c.activeSeconds + 59) / 60 }

    private func writeChatWorklog(_ c: ChatDay, key: String) throws {
        if var w = chatWorklog(c) {
            guard w.status != .exported else { return }
            w.issueKey = key
            try db.updateWorklog(w)
        } else {
            try db.insertWorklog(
                Worklog(
                    id: 0, day: c.day, start: c.times[0], duration: Self.chatMinutes(c) * 60, issueKey: key,
                    description: "Chat: \(c.name)", source: .chat, sourceRef: c.ref, repoId: nil,
                    estimated: c.activeSeconds, status: .draft, locked: true, tempoId: nil))
        }
    }

    /// "Always": the conversation is logged to `issue` on every day it has messages; nil removes the rule.
    func setChatRule(_ c: ChatDay, issue: String?) {
        settings.chatRules.removeAll { $0.conversationId == c.conversationId }
        if let key = issue.flatMap(JiraKey.normalize) {
            settings.chatRules.append(ChatRule(conversationId: c.conversationId, name: c.name, issueKey: key))
            applyChatRules()
        }
    }

    /// Logs every conversation-day in the range that has a rule and isn't logged yet. Returns how many were added.
    @discardableResult
    func applyChatRules() -> Int {
        let rules = Dictionary(settings.chatRules.map { ($0.conversationId, $0.issueKey) }, uniquingKeysWith: { a, _ in a })
        let open = chats.filter {
            rules[$0.conversationId] != nil && chatWorklog($0) == nil && $0.activeSeconds > ChatDay.minLoggable
        }
        guard !open.isEmpty else { return 0 }
        attempt("Logging chats by rule") {
            try db.transaction { for c in open { try writeChatWorklog(c, key: rules[c.conversationId]!) } }
        }
        reloadRange()
        return open.count
    }

    /// Reads your Google Chat messages for the range in the background (the signed-in Google Chat page, no window)
    /// and imports them, with progress in the status bar. Returns true when you need to sign in first.
    @discardableResult
    func fetchGoogleChat() async -> Bool {
        guard !isBusy else { return false }
        let email = settings.chatEmail.isEmpty ? (settings.authorEmails.first ?? "") : settings.chatEmail
        guard !email.isEmpty else {
            status = String(localized: "Enter your Google Chat email on the Chats page first.")
            return false
        }
        isBusy = true
        defer { isBusy = false }
        do {
            let export = try await GoogleChatCollector(session: WebSession.googleChat, email: email)
                .collect(from: rangeStart, to: rangeEnd) { self.status = $0 }
            let added = try db.importChats(export)
            reloadRange()
            applyChatRules()
            status = String(localized: "Google Chat: \(added) new messages in \(export.conversations.count) conversations.")
        } catch WebSessionError.notSignedIn {
            status = String(localized: "Google Chat is signed out. Sign in in the Google Chat window; fetching continues after.")
            signInNeeded = .googleChat
            return true
        } catch {
            status = String(localized: "Google Chat: \(error.localizedDescription)")
        }
        return false
    }

    func clearChatsInRange() {
        attempt("Clearing chats") { try db.deleteChats(from: rangeStart, to: rangeEnd) }
        reloadRange()
    }
}
