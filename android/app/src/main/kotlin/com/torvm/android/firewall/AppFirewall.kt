package com.torvm.android.firewall

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey

/**
 * Manages per-app VPN rules using Android's VpnService.Builder API.
 *
 * Instead of inspecting packets at the per-packet level, this uses the
 * platform's [addAllowedApplication] / [addDisallowedApplication] API
 * which is more reliable and efficient.
 *
 * Modes:
 * - **ALLOW_ALL**: All apps route through TorVM (default).
 * - **ALLOW_LIST**: Only specified apps route through TorVM.
 * - **BLOCK_LIST**: All apps except specified ones route through TorVM.
 */
class AppFirewall(context: Context) {

    enum class Mode {
        ALLOW_ALL,
        ALLOW_LIST,
        BLOCK_LIST
    }

    private val masterKey = MasterKey.Builder(context)
        .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
        .build()

    private val prefs: SharedPreferences = EncryptedSharedPreferences.create(
        context,
        "torvm_firewall",
        masterKey,
        EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
        EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
    )

    /** Current firewall mode. */
    var mode: Mode
        get() = Mode.entries.getOrElse(prefs.getInt("mode", 0)) { Mode.ALLOW_ALL }
        set(value) = prefs.edit().putInt("mode", value.ordinal).apply()

    /** Package names in the allow/block list depending on mode. */
    var appList: Set<String>
        get() = prefs.getStringSet("app_list", emptySet()) ?: emptySet()
        set(value) = prefs.edit().putStringSet("app_list", value).apply()

    /** Add a package to the list. */
    fun addApp(packageName: String) {
        appList = appList + packageName
    }

    /** Remove a package from the list. */
    fun removeApp(packageName: String) {
        appList = appList - packageName
    }

    /** Check if a package is in the list. */
    fun isListed(packageName: String): Boolean = packageName in appList
}
