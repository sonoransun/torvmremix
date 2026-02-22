package com.torvm.android.socks

import java.io.IOException

/**
 * Exception thrown when a SOCKS5 protocol error occurs.
 *
 * @param message a human-readable description of the error
 * @param replyCode the SOCKS5 reply code from the server, or -1 if not applicable
 * @param cause the underlying exception, if any
 */
class Socks5Exception(
    message: String,
    val replyCode: Int = -1,
    cause: Throwable? = null
) : IOException(message, cause)
