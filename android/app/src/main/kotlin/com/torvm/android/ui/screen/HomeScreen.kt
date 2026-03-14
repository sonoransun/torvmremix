package com.torvm.android.ui.screen

import androidx.biometric.BiometricManager
import androidx.biometric.BiometricPrompt
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
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.getValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import androidx.compose.ui.unit.sp
import androidx.core.content.ContextCompat
import androidx.fragment.app.FragmentActivity
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
    val biometricRequired by viewModel.biometricRequired.collectAsStateWithLifecycle()
    val context = LocalContext.current

    val toggleLabel = if (vpnState == VpnState.CONNECTED || vpnState == VpnState.CONNECTING)
        "Disconnect" else "Connect"

    // Show biometric prompt when required
    LaunchedEffect(biometricRequired) {
        if (!biometricRequired) return@LaunchedEffect
        val activity = context as? FragmentActivity ?: run {
            viewModel.startVpn(context)
            return@LaunchedEffect
        }

        val biometricManager = BiometricManager.from(context)
        val canAuth = biometricManager.canAuthenticate(BiometricManager.Authenticators.BIOMETRIC_STRONG)
        if (canAuth != BiometricManager.BIOMETRIC_SUCCESS) {
            // No biometric hardware or not enrolled; proceed without auth
            viewModel.startVpn(context)
            return@LaunchedEffect
        }

        val executor = ContextCompat.getMainExecutor(context)
        val callback = object : BiometricPrompt.AuthenticationCallback() {
            override fun onAuthenticationSucceeded(result: BiometricPrompt.AuthenticationResult) {
                viewModel.startVpn(context)
            }
            override fun onAuthenticationError(errorCode: Int, errString: CharSequence) {
                viewModel.cancelBiometric()
            }
            override fun onAuthenticationFailed() {
                // Let the user retry; do nothing
            }
        }
        val prompt = BiometricPrompt(activity, executor, callback)
        val promptInfo = BiometricPrompt.PromptInfo.Builder()
            .setTitle("Authenticate to Connect")
            .setSubtitle("Verify your identity to start the VPN")
            .setNegativeButtonText("Cancel")
            .build()
        prompt.authenticate(promptInfo)
    }

    Scaffold(
        topBar = {
            TopAppBar(
                title = { Text("TorVM") },
                colors = TopAppBarDefaults.topAppBarColors(
                    containerColor = MaterialTheme.colorScheme.primaryContainer
                ),
                actions = {
                    IconButton(
                        onClick = onNavigateToSettings,
                        modifier = Modifier.semantics { contentDescription = "Open settings" }
                    ) {
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
                    .height(56.dp)
                    .semantics { contentDescription = "$toggleLabel VPN connection" },
                colors = ButtonDefaults.buttonColors(
                    containerColor = if (vpnState == VpnState.CONNECTED || vpnState == VpnState.CONNECTING)
                        TorRed else TorPurple
                )
            ) {
                Text(
                    text = toggleLabel,
                    fontSize = 18.sp
                )
            }
            Spacer(modifier = Modifier.height(16.dp))
            TextButton(
                onClick = onNavigateToLogs,
                modifier = Modifier.semantics { contentDescription = "View connection logs" }
            ) {
                Text("View Logs")
            }
        }
    }
}
