import SwiftUI

/// First-run screen (later behind the toolbar's Modules button): pick the data sources WorkLogger uses.
struct WelcomeView: View {
    @Environment(AppState.self) private var state
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        @Bindable var state = state
        VStack(spacing: 24) {
            VStack(spacing: 6) {
                Text("Welcome to WorkLogger").font(.largeTitle.weight(.semibold))
                Text("Choose where your working time comes from. You can change this any time.")
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
                        .frame(width: 200, height: 210)
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
            Button("Continue") { dismiss() }
                .buttonStyle(.glassProminent).controlSize(.large)
                .keyboardShortcut(.defaultAction)
        }
        .padding(32)
        .onDisappear { state.settings.welcomed = true }
    }
}
