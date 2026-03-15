package com.torvm.android.tile

import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.service.quicksettings.Tile
import android.service.quicksettings.TileService
import com.torvm.android.vpn.TorVpnService
import com.torvm.android.vpn.VpnState

/**
 * Quick Settings tile for toggling TorVM VPN from the system notification shade.
 * Registered in AndroidManifest.xml as a TileService.
 */
class TorVmTileService : TileService() {

    override fun onStartListening() {
        super.onStartListening()
        updateTile()
    }

    override fun onClick() {
        super.onClick()

        val isActive = VpnState.isConnected
        if (isActive) {
            // Disconnect: send stop intent to VPN service.
            val stopIntent = Intent(this, TorVpnService::class.java).apply {
                action = TorVpnService.ACTION_DISCONNECT
            }
            startService(stopIntent)
        } else {
            // Connect: check VPN permission first.
            val prepareIntent = VpnService.prepare(this)
            if (prepareIntent != null) {
                // Need to open the app for VPN permission prompt.
                // Tiles can't show permission dialogs directly.
                val launchIntent = packageManager.getLaunchIntentForPackage(packageName)
                launchIntent?.addFlags(Intent.FLAG_ACTIVITY_NEW_TASK)
                startActivityAndCollapse(launchIntent!!)
                return
            }

            val startIntent = Intent(this, TorVpnService::class.java).apply {
                action = TorVpnService.ACTION_CONNECT
            }
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                startForegroundService(startIntent)
            } else {
                startService(startIntent)
            }
        }

        updateTile()
    }

    private fun updateTile() {
        val tile = qsTile ?: return
        val isActive = VpnState.isConnected

        tile.state = if (isActive) Tile.STATE_ACTIVE else Tile.STATE_INACTIVE
        tile.label = if (isActive) "TorVM: On" else "TorVM: Off"
        tile.subtitle = if (isActive) "Connected via Tor" else "Tap to connect"
        tile.updateTile()
    }
}
