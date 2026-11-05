import Foundation
import Security
import WorkLoggerCalendar
import WorkLoggerCore
import WorkLoggerGit
import WorkLoggerStore
import WorkLoggerTempo
import WorkLoggerWeb

/// API tokens live in the macOS Keychain, never in the SQLite file: all of them in one item, so reading them asks
/// for access once.
enum Keychain {
    private static let service = "WorkLogger"

    private static func base(_ account: String) -> [String: Any] {
        [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecAttrAccount as String: account,
        ]
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

    private static let bundle = "tokens"

    /// All tokens; nil when the Keychain refused (so nothing gets overwritten). The first time, the items from before
    /// (one per token) are moved into the single item.
    static func loadAll() -> [String: String]? {
        var one = base(bundle)
        one[kSecReturnData as String] = true
        one[kSecMatchLimit as String] = kSecMatchLimitOne
        var data: CFTypeRef?
        switch SecItemCopyMatching(one as CFDictionary, &data) {
        case errSecSuccess:
            return (data as? Data).flatMap { try? JSONDecoder().decode([String: String].self, from: $0) } ?? [:]
        case errSecItemNotFound:
            break  // first run with the single item: move the old ones in
        default:
            return nil
        }
        let q: [String: Any] = [
            kSecClass as String: kSecClassGenericPassword,
            kSecAttrService as String: service,
            kSecReturnAttributes as String: true,
            kSecMatchLimit as String: kSecMatchLimitAll,
        ]
        var out: CFTypeRef?
        var all: [String: String] = [:]
        if SecItemCopyMatching(q as CFDictionary, &out) == errSecSuccess, let items = out as? [[String: Any]] {
            for account in items.compactMap({ $0[kSecAttrAccount as String] as? String }) where account != bundle {
                all[account] = get(account)
            }
        }
        saveAll(all)
        for account in all.keys { SecItemDelete(base(account) as CFDictionary) }
        return all
    }

    static func saveAll(_ all: [String: String]) {
        set(String(decoding: (try? JSONEncoder().encode(all)) ?? Data("{}".utf8), as: UTF8.self), for: bundle)
    }
}
