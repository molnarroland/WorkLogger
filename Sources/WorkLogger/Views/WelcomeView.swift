import SwiftUI
import WorkLoggerCore
import WorkLoggerStore
import WorkLoggerGit
import WorkLoggerCalendar
import WorkLoggerTempo
import WorkLoggerWeb

/// First-run screen (later behind the toolbar's Modules button): pick the data sources WorkLogger uses.
struct WelcomeView: View {
    @Environment(AppState.self) private var state
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 24) {
            VStack(spacing: 6) {
                Text("Welcome to WorkLogger").font(.largeTitle.weight(.semibold))
                Text("Choose where your working time comes from, and how it gets into Tempo. You can change this any time with Modules.")
                    .foregroundStyle(.secondary)
            }
            HStack(alignment: .top, spacing: 16) {
                ForEach(Module.allCases) { m in
                    VStack(spacing: 12) {
                        VStack(spacing: 10) {
                            Image(systemName: m.icon).font(.system(size: 34)).frame(height: 44)
                            Text(m.title).font(.headline)
                            Text(m.summary).font(.callout).foregroundStyle(.secondary)
                                .multilineTextAlignment(.center).fixedSize(horizontal: false, vertical: true)
                            Spacer(minLength: 0)
                        }
                        .padding(16)
                        .frame(width: 180, height: 210)
                        .glassEffect(.regular, in: .rect(cornerRadius: 20))
                        Toggle(m.title, isOn: Binding(
                            get: { state.settings.modules.contains(m) },
                            set: { on in
                                if on { state.settings.modules.insert(m) } else { state.settings.modules.remove(m) }
                            }))
                        .toggleStyle(.switch).labelsHidden()
                    }
                }
            }
            VStack(alignment: .leading, spacing: 10) {
                Text("How do you log in Tempo?").font(.headline)
                Picker("How do you log in Tempo?", selection: $state.settings.tempoByHand) {
                    Text("By hand, one ticket at a time. No API key: WorkLogger lists each ticket's days to copy.").tag(true)
                    Text("Send with the Tempo API. Needs a Tempo token and Jira email in Settings.").tag(false)
                }
                .pickerStyle(.radioGroup).labelsHidden()
            }
            .padding(16)
            .frame(maxWidth: 620, alignment: .leading)
            .glassEffect(.regular, in: .rect(cornerRadius: 20))
            if state.settings.tempoByHand {
                Button("Continue") { dismiss() }
                    .buttonStyle(.glassProminent).controlSize(.large)
                    .keyboardShortcut(.defaultAction)
            } else {
                SettingsLink { Text("Continue to Tempo settings") }
                    .buttonStyle(.glassProminent).controlSize(.large)
                    .simultaneousGesture(TapGesture().onEnded { dismiss() })
            }
        }
        .padding(32)
        .onDisappear { state.settings.welcomed = true }
    }
}
