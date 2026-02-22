package com.torvm.android.vpn

import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.net.VpnService
import android.os.Build
import android.os.ParcelFileDescriptor
import androidx.core.app.NotificationCompat
import com.torvm.android.R
import com.torvm.android.data.ConnectionConfig
import com.torvm.android.data.PreferencesRepository
import com.torvm.android.dns.DnsRelay
import com.torvm.android.tcp.TcpSessionManager
import com.torvm.android.tunnel.PacketInterceptor
import com.torvm.android.tunnel.TunReader
import com.torvm.android.tunnel.TunWriter
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.Job
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.first
import kotlinx.coroutines.isActive
import kotlinx.coroutines.launch
import java.io.FileInputStream
import java.io.FileOutputStream

class TorVpnService : VpnService() {

    companion object {
        private const val VPN_ADDRESS = "10.0.0.2"
        private const val VPN_ROUTE = "0.0.0.0"
        private const val VPN_MTU = 1500
        private const val NOTIFICATION_ID = 1
        private const val CHANNEL_ID = "torvm_vpn"

        const val ACTION_START = "com.torvm.android.vpn.START"
        const val ACTION_STOP = "com.torvm.android.vpn.STOP"

        val state = MutableStateFlow(VpnState.DISCONNECTED)
        val errorMessage = MutableStateFlow<String?>(null)
    }

    private var vpnInterface: ParcelFileDescriptor? = null
    private var tunReader: TunReader? = null
    private var tunWriter: TunWriter? = null
    private var packetInterceptor: PacketInterceptor? = null
    private var tcpSessionManager: TcpSessionManager? = null
    private var dnsRelay: DnsRelay? = null
    private var readJob: Job? = null
    private val scope = CoroutineScope(Dispatchers.IO + SupervisorJob())

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_STOP -> {
                stopVpn()
                return START_NOT_STICKY
            }
            else -> {
                startVpn()
                return START_STICKY
            }
        }
    }

    private fun startVpn() {
        scope.launch {
            try {
                state.value = VpnState.CONNECTING
                errorMessage.value = null

                val config = loadConfig()
                val vpnFd = createVpnInterface(config)
                vpnInterface = vpnFd

                val input = FileInputStream(vpnFd.fileDescriptor)
                val output = FileOutputStream(vpnFd.fileDescriptor)
                tunReader = TunReader(input)
                tunWriter = TunWriter(output)

                val socketProtector: (java.net.Socket) -> Boolean = { socket ->
                    protect(socket)
                }
                val datagramProtector: (java.net.DatagramSocket) -> Boolean = { socket ->
                    protect(socket)
                }

                tcpSessionManager = TcpSessionManager(
                    config.socksHost, config.socksPort, tunWriter!!, socketProtector
                ).also { it.start() }

                dnsRelay = DnsRelay(
                    config.dnsHost, config.dnsPort, tunWriter!!, datagramProtector
                ).also { it.start() }

                packetInterceptor = PacketInterceptor(tcpSessionManager!!, dnsRelay!!)

                startForeground(NOTIFICATION_ID, createNotification())
                state.value = VpnState.CONNECTED

                readJob = scope.launch {
                    val buffer = ByteArray(VPN_MTU)
                    while (isActive) {
                        try {
                            val length = tunReader!!.read(buffer)
                            if (length > 0) {
                                val packet = buffer.copyOf(length)
                                packetInterceptor!!.intercept(packet, length)
                            } else if (length < 0) {
                                break
                            }
                        } catch (e: Exception) {
                            if (isActive) throw e
                        }
                    }
                }
            } catch (e: Exception) {
                state.value = VpnState.ERROR
                errorMessage.value = e.message ?: "Unknown error"
                stopVpn()
            }
        }
    }

    private fun stopVpn() {
        readJob?.cancel()
        tcpSessionManager?.stop()
        dnsRelay?.stop()
        vpnInterface?.close()
        vpnInterface = null
        scope.cancel()
        state.value = VpnState.DISCONNECTED
        stopForeground(STOP_FOREGROUND_REMOVE)
        stopSelf()
    }

    private fun createVpnInterface(config: ConnectionConfig): ParcelFileDescriptor {
        return Builder()
            .setSession("TorVM")
            .addAddress(VPN_ADDRESS, 32)
            .addRoute(VPN_ROUTE, 0)
            .addDnsServer(config.dnsHost)
            .setMtu(VPN_MTU)
            .setBlocking(true)
            .establish() ?: throw IllegalStateException("VPN interface creation failed")
    }

    private suspend fun loadConfig(): ConnectionConfig {
        val prefs = PreferencesRepository(applicationContext)
        return prefs.config.first()
    }

    private fun createNotification(): android.app.Notification {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
            val channel = NotificationChannel(
                CHANNEL_ID,
                getString(R.string.vpn_notification_channel),
                NotificationManager.IMPORTANCE_LOW
            )
            val nm = getSystemService(NotificationManager::class.java)
            nm.createNotificationChannel(channel)
        }
        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.vpn_notification_title))
            .setContentText(getString(R.string.vpn_notification_text))
            .setSmallIcon(R.drawable.ic_vpn_key)
            .setOngoing(true)
            .build()
    }

    override fun onDestroy() {
        stopVpn()
        super.onDestroy()
    }
}
