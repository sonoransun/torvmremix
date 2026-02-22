package com.torvm.android.data

import android.content.Context
import androidx.datastore.preferences.core.edit
import androidx.datastore.preferences.core.intPreferencesKey
import androidx.datastore.preferences.core.stringPreferencesKey
import androidx.datastore.preferences.preferencesDataStore
import kotlinx.coroutines.flow.Flow
import kotlinx.coroutines.flow.map

class PreferencesRepository(private val context: Context) {

    private val Context.dataStore by preferencesDataStore(name = "torvm_settings")

    private object Keys {
        val SOCKS_HOST = stringPreferencesKey("socks_host")
        val SOCKS_PORT = intPreferencesKey("socks_port")
        val DNS_HOST = stringPreferencesKey("dns_host")
        val DNS_PORT = intPreferencesKey("dns_port")
    }

    val config: Flow<ConnectionConfig> = context.dataStore.data.map { prefs ->
        ConnectionConfig(
            socksHost = prefs[Keys.SOCKS_HOST] ?: ConnectionConfig.DIRECT.socksHost,
            socksPort = prefs[Keys.SOCKS_PORT] ?: ConnectionConfig.DIRECT.socksPort,
            dnsHost = prefs[Keys.DNS_HOST] ?: ConnectionConfig.DIRECT.dnsHost,
            dnsPort = prefs[Keys.DNS_PORT] ?: ConnectionConfig.DIRECT.dnsPort
        )
    }

    suspend fun saveConfig(config: ConnectionConfig) {
        context.dataStore.edit { prefs ->
            prefs[Keys.SOCKS_HOST] = config.socksHost
            prefs[Keys.SOCKS_PORT] = config.socksPort
            prefs[Keys.DNS_HOST] = config.dnsHost
            prefs[Keys.DNS_PORT] = config.dnsPort
        }
    }
}
