package com.torvm.android.data

import android.content.Context
import android.content.SharedPreferences
import androidx.security.crypto.EncryptedSharedPreferences
import androidx.security.crypto.MasterKey
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.map
import kotlinx.coroutines.withContext

/**
 * Stores connection settings in encrypted shared preferences.
 *
 * Uses AndroidX Security Crypto [EncryptedSharedPreferences] backed by the
 * Android Keystore, ensuring that SOCKS/DNS host and port values are
 * encrypted at rest.
 */
class PreferencesRepository(private val context: Context) {

    private object Keys {
        const val SOCKS_HOST = "socks_host"
        const val SOCKS_PORT = "socks_port"
        const val DNS_HOST = "dns_host"
        const val DNS_PORT = "dns_port"
    }

    private val masterKey: MasterKey by lazy {
        MasterKey.Builder(context)
            .setKeyScheme(MasterKey.KeyScheme.AES256_GCM)
            .build()
    }

    private val encryptedPrefs: SharedPreferences by lazy {
        EncryptedSharedPreferences.create(
            context,
            "torvm_encrypted_settings",
            masterKey,
            EncryptedSharedPreferences.PrefKeyEncryptionScheme.AES256_SIV,
            EncryptedSharedPreferences.PrefValueEncryptionScheme.AES256_GCM
        )
    }

    private val _configFlow = MutableStateFlow(readConfig())

    /** Observe configuration changes as a Flow. */
    val config: Flow<ConnectionConfig> = _configFlow

    private fun readConfig(): ConnectionConfig {
        val defaults = ConnectionConfig.DIRECT
        return ConnectionConfig(
            socksHost = encryptedPrefs.getString(Keys.SOCKS_HOST, defaults.socksHost) ?: defaults.socksHost,
            socksPort = encryptedPrefs.getInt(Keys.SOCKS_PORT, defaults.socksPort),
            dnsHost = encryptedPrefs.getString(Keys.DNS_HOST, defaults.dnsHost) ?: defaults.dnsHost,
            dnsPort = encryptedPrefs.getInt(Keys.DNS_PORT, defaults.dnsPort)
        )
    }

    suspend fun saveConfig(config: ConnectionConfig) = withContext(Dispatchers.IO) {
        encryptedPrefs.edit()
            .putString(Keys.SOCKS_HOST, config.socksHost)
            .putInt(Keys.SOCKS_PORT, config.socksPort)
            .putString(Keys.DNS_HOST, config.dnsHost)
            .putInt(Keys.DNS_PORT, config.dnsPort)
            .apply()
        _configFlow.value = config
    }
}
