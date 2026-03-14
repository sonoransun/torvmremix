package com.torvm.android.ui.component

import androidx.compose.animation.animateColorAsState
import androidx.compose.foundation.Canvas
import androidx.compose.foundation.layout.size
import androidx.compose.runtime.Composable
import androidx.compose.runtime.getValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.semantics.contentDescription
import androidx.compose.ui.semantics.semantics
import androidx.compose.ui.unit.dp
import com.torvm.android.ui.theme.TorGreen
import com.torvm.android.ui.theme.TorRed
import com.torvm.android.ui.theme.TorYellow
import com.torvm.android.vpn.VpnState

@Composable
fun StatusIndicator(
    state: VpnState,
    modifier: Modifier = Modifier
) {
    val color = when (state) {
        VpnState.CONNECTED -> TorGreen
        VpnState.CONNECTING -> TorYellow
        VpnState.DISCONNECTED -> Color.Gray
        VpnState.ERROR -> TorRed
    }
    val statusDescription = when (state) {
        VpnState.CONNECTED -> "Connection status: Connected"
        VpnState.CONNECTING -> "Connection status: Connecting"
        VpnState.DISCONNECTED -> "Connection status: Disconnected"
        VpnState.ERROR -> "Connection status: Error"
    }
    // Animate color changes
    val animatedColor by animateColorAsState(targetValue = color, label = "status")

    Canvas(
        modifier = modifier
            .size(24.dp)
            .semantics { contentDescription = statusDescription }
    ) {
        drawCircle(color = animatedColor)
    }
}
