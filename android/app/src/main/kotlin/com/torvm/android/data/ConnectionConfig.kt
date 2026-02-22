package com.torvm.android.data

data class ConnectionConfig(
    val socksHost: String = "10.10.10.1",
    val socksPort: Int = 9050,
    val dnsHost: String = "10.10.10.1",
    val dnsPort: Int = 9093
) {
    companion object {
        val DIRECT = ConnectionConfig()
        val WIFI_DEFAULT = ConnectionConfig(
            socksHost = "192.168.1.100",
            socksPort = 9050,
            dnsHost = "192.168.1.100",
            dnsPort = 9093
        )
    }
}
