import SwiftUI

struct StatusIndicator: View {
    let state: VPNConnectionState

    private var color: Color {
        switch state {
        case .connected: return .green
        case .connecting: return .yellow
        case .disconnected: return .gray
        case .disconnecting: return .orange
        case .error: return .red
        }
    }

    var body: some View {
        Circle()
            .fill(color)
            .shadow(color: color.opacity(0.5), radius: 8)
            .animation(.easeInOut(duration: 0.4), value: state)
    }
}

#Preview {
    VStack(spacing: 20) {
        StatusIndicator(state: .connected)
            .frame(width: 80, height: 80)
        StatusIndicator(state: .connecting)
            .frame(width: 80, height: 80)
        StatusIndicator(state: .disconnected)
            .frame(width: 80, height: 80)
    }
}
