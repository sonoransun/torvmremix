import NetworkExtension
import os.log

/// NEPacketTunnelProvider subclass that implements the TorVM VPN tunnel.
///
/// This is the iOS equivalent of Android's TorVpnService. It runs in a separate
/// process from the main app and communicates via App Group Keychain and shared files.
///
/// The tunnel reads packets from the virtual TUN interface via packetFlow,
/// dispatches them through the userspace TCP/IP stack (PacketInterceptor →
/// TcpSessionManager / DnsRelay), and writes response packets back.
class PacketTunnelProvider: NEPacketTunnelProvider {

    private let logger = Logger(subsystem: "com.torvm.ios.tunnel", category: "PacketTunnel")

    private var tcpSessionManager: TcpSessionManager?
    private var dnsRelay: DnsRelay?
    private var packetInterceptor: PacketInterceptor?
    private var isRunning = false

    private static let vpnAddress = "10.0.0.2"
    private static let vpnSubnetMask = "255.255.255.255"
    private static let vpnMTU: NSNumber = 1500

    // MARK: - Tunnel Lifecycle

    override func startTunnel(
        options: [String: NSObject]?,
        completionHandler: @escaping (Error?) -> Void
    ) {
        logger.info("Starting TorVM tunnel")

        // Load config from options (fast path) or shared Keychain (fallback).
        let config: ConnectionConfig
        do {
            if let configData = options?["config"] as? Data,
               let optConfig = try? JSONDecoder().decode(ConnectionConfig.self, from: configData) {
                config = optConfig
            } else {
                config = try KeychainManager.shared.loadConfig()
            }
        } catch {
            logger.error("Failed to load config: \(error.localizedDescription)")
            completionHandler(error)
            return
        }

        // Configure tunnel network settings.
        let networkSettings = NEPacketTunnelNetworkSettings(
            tunnelRemoteAddress: config.socksHost
        )

        let ipv4Settings = NEIPv4Settings(
            addresses: [Self.vpnAddress],
            subnetMasks: [Self.vpnSubnetMask]
        )
        ipv4Settings.includedRoutes = [NEIPv4Route.default()]
        // Exclude the proxy host from the tunnel to prevent routing loops.
        // This replaces Android's VpnService.protect(socket) mechanism.
        ipv4Settings.excludedRoutes = [
            NEIPv4Route(
                destinationAddress: config.socksHost,
                subnetMask: "255.255.255.255"
            )
        ]
        networkSettings.ipv4Settings = ipv4Settings

        networkSettings.dnsSettings = NEDNSSettings(servers: [config.dnsHost])
        networkSettings.mtu = Self.vpnMTU

        setTunnelNetworkSettings(networkSettings) { [weak self] error in
            guard let self = self else { return }

            if let error = error {
                self.logger.error("Failed to set tunnel settings: \(error.localizedDescription)")
                completionHandler(error)
                return
            }

            // Initialize the userspace TCP/IP stack.
            let packetWriter: (Data) -> Void = { [weak self] packet in
                self?.packetFlow.writePackets(
                    [packet],
                    withProtocols: [NSNumber(value: AF_INET)]
                )
            }

            self.tcpSessionManager = TcpSessionManager(
                socksHost: config.socksHost,
                socksPort: config.socksPort,
                packetWriter: packetWriter
            )

            self.dnsRelay = DnsRelay(
                dnsHost: config.dnsHost,
                dnsPort: config.dnsPort,
                packetFlow: self.packetFlow
            )
            self.dnsRelay?.start()

            self.packetInterceptor = PacketInterceptor(
                tcpSessionManager: self.tcpSessionManager!,
                dnsRelay: self.dnsRelay!
            )

            self.isRunning = true
            self.startReadingPackets()

            TunnelLogger.shared.log(
                level: "info",
                message: "Tunnel started: SOCKS5=\(config.socksHost):\(config.socksPort), DNS=\(config.dnsHost):\(config.dnsPort)",
                source: "tunnel"
            )
            self.logger.info("TorVM tunnel started successfully")
            completionHandler(nil)
        }
    }

    override func stopTunnel(
        with reason: NEProviderStopReason,
        completionHandler: @escaping () -> Void
    ) {
        logger.info("Stopping TorVM tunnel, reason: \(String(describing: reason))")
        isRunning = false
        tcpSessionManager?.stop()
        dnsRelay?.stop()

        TunnelLogger.shared.log(
            level: "info",
            message: "Tunnel stopped",
            source: "tunnel"
        )
        completionHandler()
    }

    // MARK: - Packet Reading

    /// Read packets from the TUN device and dispatch them via PacketInterceptor.
    ///
    /// NEPacketTunnelFlow.readPackets is callback-based; we call it again
    /// recursively to continue receiving packets.
    private func startReadingPackets() {
        packetFlow.readPackets { [weak self] packets, protocols in
            guard let self = self, self.isRunning else { return }

            for (i, packetData) in packets.enumerated() {
                let proto = protocols[i].int32Value
                guard proto == AF_INET else { continue }

                self.packetInterceptor?.intercept(
                    packet: packetData,
                    length: packetData.count
                )
            }

            // Continue reading.
            self.startReadingPackets()
        }
    }

    // MARK: - Sleep/Wake

    override func sleep(completionHandler: @escaping () -> Void) {
        completionHandler()
    }

    override func wake() {
        if isRunning {
            startReadingPackets()
        }
    }
}
