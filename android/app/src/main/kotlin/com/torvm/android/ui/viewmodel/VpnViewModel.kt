package com.torvm.android.ui.viewmodel

import android.app.Application
import android.content.Context
import android.content.Intent
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.torvm.android.data.ConnectionConfig
import com.torvm.android.data.PreferencesRepository
import com.torvm.android.vpn.TorVpnService
import com.torvm.android.vpn.VpnState
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn

class VpnViewModel(application: Application) : AndroidViewModel(application) {
    private val prefsRepo = PreferencesRepository(application)

    val vpnState: StateFlow<VpnState> = TorVpnService.state
    val errorMessage: StateFlow<String?> = TorVpnService.errorMessage
    val config: StateFlow<ConnectionConfig> = prefsRepo.config
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), ConnectionConfig.DIRECT)

    fun toggleVpn(context: Context) {
        val intent = Intent(context, TorVpnService::class.java)
        when (vpnState.value) {
            VpnState.CONNECTED, VpnState.CONNECTING -> {
                intent.action = TorVpnService.ACTION_STOP
                context.startService(intent)
            }
            else -> {
                intent.action = TorVpnService.ACTION_START
                context.startForegroundService(intent)
            }
        }
    }
}
