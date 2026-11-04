import Foundation
import Security

enum SecretKey {
    static let jiraToken = "jira.apiToken"
    static let tempoToken = "tempo.apiToken"
    static let googleSecret = "google.clientSecret"
    static let googleRefresh = "google.refreshToken"
}

/// API tokens live in the macOS Keychain, never in the SQLite file.
enum Keychain {
    private static let service = "WorkLogger"

    private static func base(_ account: String) -> [String: Any] {
        [kSecClass as String: kSecClassGenericPassword,
         kSecAttrService as String: service,
         kSecAttrAccount as String: account]
    }

    static func set(_ value: String?, for account: String) {
        SecItemDelete(base(account) as CFDictionary)
        guard let value, !value.isEmpty else { return }
        var add = base(account)
        add[kSecValueData as String] = Data(value.utf8)
        SecItemAdd(add as CFDictionary, nil)
    }

    static func get(_ account: String) -> String? {
        var q = base(account)
        q[kSecReturnData as String] = true
        q[kSecMatchLimit as String] = kSecMatchLimitOne
        var out: CFTypeRef?
        guard SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let data = out as? Data else { return nil }
        return String(data: data, encoding: .utf8)
    }
}
