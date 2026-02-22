package com.torvm.android.ui.viewmodel

import android.app.Application
import androidx.lifecycle.AndroidViewModel
import androidx.lifecycle.viewModelScope
import com.torvm.android.data.ConnectionConfig
import com.torvm.android.data.PreferencesRepository
import kotlinx.coroutines.flow.SharingStarted
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.stateIn
import kotlinx.coroutines.launch

class SettingsViewModel(application: Application) : AndroidViewModel(application) {
    private val prefsRepo = PreferencesRepository(application)

    val config: StateFlow<ConnectionConfig> = prefsRepo.config
        .stateIn(viewModelScope, SharingStarted.WhileSubscribed(5000), ConnectionConfig.DIRECT)

    fun save(config: ConnectionConfig) {
        viewModelScope.launch {
            prefsRepo.saveConfig(config)
        }
    }
}
