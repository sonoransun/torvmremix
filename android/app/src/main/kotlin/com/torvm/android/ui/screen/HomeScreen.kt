package com.torvm.android.ui.screen

import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.Button
import androidx.compose.material3.ButtonDefaults
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.lifecycle.compose.collectAsStateWithLifecycle
import androidx.lifecycle.viewmodel.compose.viewModel
import com.torvm.android.ui.component.ConnectionCard
import com.torvm.android.ui.component.StatusIndicator
import com.torvm.android.ui.theme.TorPurple
import com.torvm.android.ui.theme.TorRed
import com.torvm.android.ui.viewmodel.VpnViewModel
import com.torvm.android.vpn.VpnState

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun HomeScreen(
    viewModel: VpnViewModel = viewModel(),
    onNavigateToSettings: () -> Unit,
    onNavigateToLogs: () -> Unit
) {
    val vpnState by viewModel.vpnState.collectAsStateWithLifecycle()
    val config by viewModel.config.collectAsStateWithLifecycle()
    val errorMessage by viewModel.errorMessage.collectAsStateWithLifecycle()
    val context = LocalContext.current

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("TorVM") },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer
                ),
                actions = {
                    IconButton(onClick = onNavigateToSettings) {
                        Icon(Icons.Default.Settings, contentDescription = "Settings")
                    }
                }
            )
        }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(16.dp),
            horizontalAlignment = Alignment.CenterHorizontally,
            verticalArrangement = Arrangement.Center
        ) {
            StatusIndicator(
                state = vpnState,
                modifier = Modifier.size(80.dp)
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = when (vpnState) {
                    VpnState.DISCONNECTED -> "Disconnected"
                    VpnState.CONNECTING -> "Connecting\u2026"
                    VpnState.CONNECTED -> "Connected"
                    VpnState.ERROR -> "Error"
                },
                style = MaterialTheme.typography.headlineMedium
            )
            if (errorMessage != null) {
                Spacer(modifier = Modifier.height(8.dp))
                Text(
                    text = errorMessage!!,
                    color = TorRed,
                    style = MaterialTheme.typography.bodyMedium
                )
            }
            Spacer(modifier = Modifier.height(24.dp))
            ConnectionCard(config = config)
            Spacer(modifier = Modifier.height(24.dp))
            Button(
                onClick = { viewModel.toggleVpn(context) },
                modifier = Modifier
                    .fillMaxWidth()
                    .padding(horizontal = 16.dp)
                    .height(56.dp),
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (vpnState == VpnState.CONNECTED || vpnState == VpnState.CONNECTING)
                        TorRed else TorPurple
                )
            ) {
                Text(
                    text = if (vpnState == VpnState.CONNECTED || vpnState == VpnState.CONNECTING)
                        "Disconnect" else "Connect",
                    fontSize = 18.sp
                )
            }
            Spacer(modifier = Modifier.height(16.dp))
            TextButton(onClick = onNavigateToLogs) {
                Text("View Logs")
            }
        }
    }
}
