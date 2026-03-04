package com.torvm.android.tcp

/**
 * TCP Reno-style congestion controller.
 *
 * Manages the congestion window (cwnd) and slow-start threshold (ssthresh)
 * for a TCP session. This limits how much data the session sends ahead of
 * acknowledgements, preventing Tor circuit congestion.
 *
 * Phases:
 * - **Slow start**: cwnd < ssthresh → cwnd grows by MSS on each ACK
 * - **Congestion avoidance**: cwnd >= ssthresh → cwnd grows by MSS²/cwnd on each ACK
 * - **Loss recovery**: on timeout → ssthresh = cwnd/2, cwnd = MSS
 */
class CongestionController(
    /** Maximum segment size in bytes. */
    private val mss: Int = TcpSession.MSS
) {
    /** Congestion window in bytes. Starts at 2*MSS (RFC 3390). */
    var cwnd: Int = 2 * mss
        private set

    /** Slow-start threshold. Initially large to allow slow start. */
    var ssthresh: Int = 65535
        private set

    /** Bytes currently in flight (sent but not yet acknowledged). */
    var bytesInFlight: Int = 0
        private set

    /** Smoothed round-trip time in milliseconds (Jacobson's algorithm). */
    var smoothedRtt: Long = 0
        private set

    /** RTT variation in milliseconds. */
    var rttVar: Long = 0
        private set

    /** Retransmission timeout in milliseconds. */
    var rto: Long = 3000  // RFC 6298 initial RTO
        private set

    private var rttInitialized = false

    /** Effective window: min of cwnd and WINDOW_SIZE (peer's receive window). */
    fun getWindowSize(): Int = minOf(cwnd, TcpSession.WINDOW_SIZE)

    /** Returns true if more data can be sent given current congestion state. */
    fun canSend(bytes: Int): Boolean = bytesInFlight + bytes <= cwnd

    /** Record that [bytes] have been sent. */
    fun onSend(bytes: Int) {
        bytesInFlight += bytes
    }

    /**
     * Process an ACK acknowledging [ackedBytes] bytes.
     * Grows cwnd according to slow-start or congestion avoidance.
     */
    fun onAck(ackedBytes: Int) {
        bytesInFlight = maxOf(0, bytesInFlight - ackedBytes)

        if (cwnd < ssthresh) {
            // Slow start: exponential growth.
            cwnd += minOf(ackedBytes, mss)
        } else {
            // Congestion avoidance: linear growth (approximately +1 MSS per RTT).
            cwnd += maxOf(1, (mss.toLong() * mss / cwnd).toInt())
        }
    }

    /**
     * Update RTT estimate using Jacobson's algorithm (RFC 6298).
     *
     * @param measuredRtt measured round-trip time in milliseconds
     */
    fun updateRtt(measuredRtt: Long) {
        if (!rttInitialized) {
            smoothedRtt = measuredRtt
            rttVar = measuredRtt / 2
            rttInitialized = true
        } else {
            // RTTVAR = (1 - 1/4) * RTTVAR + 1/4 * |SRTT - R'|
            rttVar = (3 * rttVar + Math.abs(smoothedRtt - measuredRtt)) / 4
            // SRTT = (1 - 1/8) * SRTT + 1/8 * R'
            smoothedRtt = (7 * smoothedRtt + measuredRtt) / 8
        }

        // RTO = SRTT + max(G, 4*RTTVAR), where G = clock granularity (~1ms)
        rto = smoothedRtt + maxOf(1, 4 * rttVar)
        // Clamp RTO between 200ms and 60s
        rto = rto.coerceIn(200, 60_000)
    }

    /**
     * Handle a timeout event. Cuts ssthresh to cwnd/2 and resets cwnd to MSS.
     */
    fun onTimeout() {
        ssthresh = maxOf(cwnd / 2, 2 * mss)
        cwnd = mss
        bytesInFlight = 0
        // Double RTO on timeout (Karn's algorithm)
        rto = minOf(rto * 2, 60_000)
    }

    /**
     * Handle a duplicate ACK (fast retransmit/recovery).
     * After 3 duplicate ACKs, reduce ssthresh and cwnd.
     */
    fun onTripleDuplicateAck() {
        ssthresh = maxOf(cwnd / 2, 2 * mss)
        cwnd = ssthresh + 3 * mss  // Inflate for packets in flight
    }

    /** Reset to initial state. */
    fun reset() {
        cwnd = 2 * mss
        ssthresh = 65535
        bytesInFlight = 0
        smoothedRtt = 0
        rttVar = 0
        rto = 3000
        rttInitialized = false
    }
}
