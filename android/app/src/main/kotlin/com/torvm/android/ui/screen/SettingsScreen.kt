package com.torvm.android.ui.screen

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material3.Button
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.unit.dp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.torvm.android.data.ConnectionConfig
import com.torvm.android.ui.viewmodel.SettingsViewModel

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsScreen(
    viewModel: SettingsViewModel = viewModel(),
    onNavigateBack: () -> Unit
) {
    val config by viewModel.config.collectAsStateWithLifecycle()

    var socksHost by remember(config) { mutableStateOf(config.socksHost) }
    var socksPort by remember(config) { mutableStateOf(config.socksPort.toString()) }
    var dnsHost by remember(config) { mutableStateOf(config.dnsHost) }
    var dnsPort by remember(config) { mutableStateOf(config.dnsPort.toString()) }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(onClick = onNavigateBack) {
                        Icon(Icons.AutoMirrored.Filled.ArrowBack, contentDescription = "Back")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp)
                .verticalScroll(rememberScrollState())
        ) {
            Text("Presets", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(8.dp))
            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                OutlinedButton(onClick = {
                    val d = ConnectionConfig.DIRECT
                    socksHost = d.socksHost; socksPort = d.socksPort.toString()
                    dnsHost = d.dnsHost; dnsPort = d.dnsPort.toString()
                }) { Text("Direct (TAP)") }
                OutlinedButton(onClick = {
                    val w = ConnectionConfig.WIFI_DEFAULT
                    socksHost = w.socksHost; socksPort = w.socksPort.toString()
                    dnsHost = w.dnsHost; dnsPort = w.dnsPort.toString()
                }) { Text("WiFi/LAN") }
            }
            Spacer(modifier = Modifier.height(24.dp))
            Text("Connection", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = socksHost, onValueChange = { socksHost = it },
                label = { Text("SOCKS5 Host") },
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = socksPort, onValueChange = { socksPort = it },
                label = { Text("SOCKS5 Port") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = dnsHost, onValueChange = { dnsHost = it },
                label = { Text("DNS Host") },
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedTextField(
                value = dnsPort, onValueChange = { dnsPort = it },
                label = { Text("DNS Port") },
                keyboardOptions = KeyboardOptions(keyboardType = KeyboardType.Number),
                modifier = Modifier.fillMaxWidth()
            )
            Spacer(modifier = Modifier.height(24.dp))
            Button(
                onClick = {
                    viewModel.save(ConnectionConfig(
                        socksHost = socksHost,
                        socksPort = socksPort.toIntOrNull() ?: 9050,
                        dnsHost = dnsHost,
                        dnsPort = dnsPort.toIntOrNull() ?: 9093
                    ))
                    onNavigateBack()
                },
                modifier = Modifier.fillMaxWidth()
            ) { Text("Save") }
        }
    }
}
