import Foundation

/// TCP Reno-style congestion controller.
///
/// Manages the congestion window (cwnd) and slow-start threshold (ssthresh)
/// for a TCP session. This limits how much data the session sends ahead of
/// acknowledgements, preventing Tor circuit congestion.
///
/// Phases:
/// - **Slow start**: cwnd < ssthresh -> cwnd grows by MSS on each ACK
/// - **Congestion avoidance**: cwnd >= ssthresh -> cwnd grows by MSS^2/cwnd on each ACK
/// - **Loss recovery**: on timeout -> ssthresh = cwnd/2, cwnd = MSS
struct CongestionController {
    /// Maximum segment size in bytes.
    private let mss: Int

    /// Congestion window in bytes. Starts at 2*MSS (RFC 3390).
    private(set) var cwnd: Int

    /// Slow-start threshold. Initially large to allow slow start.
    private(set) var ssthresh: Int = 65535

    /// Bytes currently in flight (sent but not yet acknowledged).
    private(set) var bytesInFlight: Int = 0

    /// Smoothed round-trip time in milliseconds (Jacobson's algorithm).
    private(set) var smoothedRtt: Int64 = 0

    /// RTT variation in milliseconds.
    private(set) var rttVar: Int64 = 0

    /// Retransmission timeout in milliseconds.
    private(set) var rto: Int64 = 3000  // RFC 6298 initial RTO

    private var rttInitialized = false

    /// Default MSS matching TcpSession.mss.
    static let defaultMSS = 1400
    /// Default window size matching TcpSession.windowSize.
    static let defaultWindowSize = 65535

    init(mss: Int = CongestionController.defaultMSS) {
        self.mss = mss
        self.cwnd = 2 * mss
    }

    /// Effective window: min of cwnd and peer's receive window.
    func getWindowSize() -> Int {
        return min(cwnd, Self.defaultWindowSize)
    }

    /// Returns true if more data can be sent given current congestion state.
    func canSend(bytes: Int) -> Bool {
        return bytesInFlight + bytes <= cwnd
    }

    /// Record that `bytes` have been sent.
    mutating func onSend(_ bytes: Int) {
        bytesInFlight += bytes
    }

    /// Process an ACK acknowledging `ackedBytes` bytes.
    /// Grows cwnd according to slow-start or congestion avoidance.
    mutating func onAck(_ ackedBytes: Int) {
        bytesInFlight = max(0, bytesInFlight - ackedBytes)

        if cwnd < ssthresh {
            // Slow start: exponential growth.
            cwnd += min(ackedBytes, mss)
        } else {
            // Congestion avoidance: linear growth (approximately +1 MSS per RTT).
            cwnd += max(1, (mss * mss) / cwnd)
        }
    }

    /// Update RTT estimate using Jacobson's algorithm (RFC 6298).
    ///
    /// - Parameter measuredRtt: measured round-trip time in milliseconds
    mutating func updateRtt(_ measuredRtt: Int64) {
        if !rttInitialized {
            smoothedRtt = measuredRtt
            rttVar = measuredRtt / 2
            rttInitialized = true
        } else {
            // RTTVAR = (1 - 1/4) * RTTVAR + 1/4 * |SRTT - R'|
            rttVar = (3 * rttVar + abs(smoothedRtt - measuredRtt)) / 4
            // SRTT = (1 - 1/8) * SRTT + 1/8 * R'
            smoothedRtt = (7 * smoothedRtt + measuredRtt) / 8
        }

        // RTO = SRTT + max(G, 4*RTTVAR), where G = clock granularity (~1ms)
        rto = smoothedRtt + max(1, 4 * rttVar)
        // Clamp RTO between 200ms and 60s
        rto = min(max(rto, 200), 60_000)
    }

    /// Handle a timeout event. Cuts ssthresh to cwnd/2 and resets cwnd to MSS.
    mutating func onTimeout() {
        ssthresh = max(cwnd / 2, 2 * mss)
        cwnd = mss
        bytesInFlight = 0
        // Double RTO on timeout (Karn's algorithm)
        rto = min(rto * 2, 60_000)
    }

    /// Handle a duplicate ACK (fast retransmit/recovery).
    /// After 3 duplicate ACKs, reduce ssthresh and cwnd.
    mutating func onTripleDuplicateAck() {
        ssthresh = max(cwnd / 2, 2 * mss)
        cwnd = ssthresh + 3 * mss  // Inflate for packets in flight
    }

    /// Reset to initial state.
    mutating func reset() {
        cwnd = 2 * mss
        ssthresh = 65535
        bytesInFlight = 0
        smoothedRtt = 0
        rttVar = 0
        rto = 3000
        rttInitialized = false
    }
}
