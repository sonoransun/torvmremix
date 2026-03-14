package com.torvm.android.ui.screen

import android.content.Context
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
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedButton
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBar
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
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
    onNavigateBack: () -> Unit,
    onNavigateToFirewall: () -> Unit = {}
) {
    val config by viewModel.config.collectAsStateWithLifecycle()
    val context = LocalContext.current

    var socksHost by remember(config) { mutableStateOf(config.socksHost) }
    var socksPort by remember(config) { mutableStateOf(config.socksPort.toString()) }
    var dnsHost by remember(config) { mutableStateOf(config.dnsHost) }
    var dnsPort by remember(config) { mutableStateOf(config.dnsPort.toString()) }

    val biometricPrefs = remember {
        context.getSharedPreferences("torvm_biometric", Context.MODE_PRIVATE)
    }
    var requireBiometric by remember {
        mutableStateOf(biometricPrefs.getBoolean("require_biometric", false))
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("Settings") },
                navigationIcon = {
                    IconButton(
                        onClick = onNavigateBack,
                        modifier = Modifier.semantics { contentDescription = "Navigate back" }
                    ) {
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
                OutlinedButton(
                    onClick = {
                        val d = ConnectionConfig.DIRECT
                        socksHost = d.socksHost; socksPort = d.socksPort.toString()
                        dnsHost = d.dnsHost; dnsPort = d.dnsPort.toString()
                    },
                    modifier = Modifier.semantics { contentDescription = "Apply Direct TAP preset" }
                ) { Text("Direct (TAP)") }
                OutlinedButton(
                    onClick = {
                        val w = ConnectionConfig.WIFI_DEFAULT
                        socksHost = w.socksHost; socksPort = w.socksPort.toString()
                        dnsHost = w.dnsHost; dnsPort = w.dnsPort.toString()
                    },
                    modifier = Modifier.semantics { contentDescription = "Apply WiFi LAN preset" }
                ) { Text("WiFi/LAN") }
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
            HorizontalDivider()
            Spacer(modifier = Modifier.height(16.dp))

            Text("Security", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(8.dp))
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Text(
                    "Require biometric to connect",
                    style = MaterialTheme.typography.bodyLarge
                )
                Switch(
                    checked = requireBiometric,
                    onCheckedChange = { checked ->
                        requireBiometric = checked
                        biometricPrefs.edit()
                            .putBoolean("require_biometric", checked)
                            .apply()
                    },
                    modifier = Modifier.semantics {
                        contentDescription = "Require biometric authentication to connect"
                    }
                )
            }

            Spacer(modifier = Modifier.height(24.dp))
            HorizontalDivider()
            Spacer(modifier = Modifier.height(16.dp))

            Text("App Firewall", style = MaterialTheme.typography.titleMedium)
            Spacer(modifier = Modifier.height(8.dp))
            OutlinedButton(
                onClick = onNavigateToFirewall,
                modifier = Modifier
                    .fillMaxWidth()
                    .semantics { contentDescription = "Open app firewall settings" }
            ) { Text("Manage Per-App Rules") }

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
                modifier = Modifier
                    .fillMaxWidth()
                    .semantics { contentDescription = "Save settings" }
            ) { Text("Save") }
        }
    }
}
