import SwiftUI
import WorkLoggerCore
import WorkLoggerStore

/// One color per source on every screen: coding blue, meetings violet, chats teal, leave pink;
/// orange always means "needs a ticket".
extension WorklogSource {
    var color: Color {
        switch self {
        case .git: return .blue
        case .meeting: return .purple
        case .chat: return .teal
        case .absence: return .pink
        case .period: return .indigo
        case .manual: return .gray
        }
    }
}

/// A day's (or month's) time as one bar split by source; time without a ticket is drawn orange.
struct SourceStack: View {
    let logs: [Worklog]
    /// The bar's full width in seconds (the day's target); the bar is never shorter than what was logged.
    var scale: Int = 0
    var height: CGFloat = 5

    var body: some View {
        let missing = logs.missingIssue.totalDuration
        let parts = logs.filter { !$0.needsIssue }.totalsBySource.map { ($0.source.color, $0.seconds) } + [(Color.orange, missing)]
        let full = max(scale, logs.totalDuration, 1)
        GeometryReader { geo in
            HStack(spacing: 0) {
                ForEach(Array(parts.enumerated()), id: \.offset) { _, part in
                    part.0.frame(width: geo.size.width * CGFloat(part.1) / CGFloat(full))
                }
                Spacer(minLength: 0)
            }
        }
        .frame(height: height)
        .background(.quaternary)
        .clipShape(.capsule)
    }
}
