import SwiftUI

struct StatusIndicator: View {
    let state: VPNConnectionState

    @State private var isPulsing = false
    @State private var shakeOffset: CGFloat = 0

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
            .shadow(color: color.opacity(glowOpacity), radius: glowRadius)
            .scaleEffect(isPulsing ? 1.15 : 1.0)
            .opacity(isPulsing ? 0.7 : 1.0)
            .offset(x: shakeOffset)
            .animation(.easeInOut(duration: 0.4), value: state)
            .onChange(of: state) { newState in
                updateAnimation(for: newState)
            }
            .onAppear {
                updateAnimation(for: state)
            }
    }

    private var glowOpacity: Double {
        switch state {
        case .connected: return 0.6
        case .error: return 0.7
        default: return 0.3
        }
    }

    private var glowRadius: CGFloat {
        switch state {
        case .connected: return 12
        case .error: return 10
        default: return 8
        }
    }

    private func updateAnimation(for newState: VPNConnectionState) {
        switch newState {
        case .connecting, .disconnecting:
            withAnimation(.easeInOut(duration: 0.8).repeatForever(autoreverses: true)) {
                isPulsing = true
            }
            shakeOffset = 0
        case .error:
            isPulsing = false
            triggerShake()
        default:
            withAnimation(.easeInOut(duration: 0.3)) {
                isPulsing = false
            }
            shakeOffset = 0
        }
    }

    private func triggerShake() {
        withAnimation(.interpolatingSpring(stiffness: 300, damping: 8)) {
            shakeOffset = 10
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) {
            withAnimation(.interpolatingSpring(stiffness: 300, damping: 8)) {
                shakeOffset = -8
            }
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
            withAnimation(.interpolatingSpring(stiffness: 300, damping: 8)) {
                shakeOffset = 5
            }
        }
        DispatchQueue.main.asyncAfter(deadline: .now() + 0.3) {
            withAnimation(.interpolatingSpring(stiffness: 300, damping: 8)) {
                shakeOffset = 0
            }
        }
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
        StatusIndicator(state: .error)
            .frame(width: 80, height: 80)
    }
}
