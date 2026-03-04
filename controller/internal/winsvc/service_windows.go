//go:build windows

package winsvc

import (
	"context"
	"fmt"
	"os"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/eventlog"
	"golang.org/x/sys/windows/svc/mgr"

	"github.com/user/extorvm/controller/internal/config"
	"github.com/user/extorvm/controller/internal/lifecycle"
	"github.com/user/extorvm/controller/internal/logging"
)

const serviceName = "TorVM"
const serviceDescription = "TorVM - Transparent Tor Network Routing"

// TorVMService implements svc.Handler for the Windows Service Control Manager.
type TorVMService struct {
	Config *config.Config
	Logger *logging.Logger
}

// Execute is called by the Windows service manager. It reports status
// transitions and runs the lifecycle engine until a stop signal is received.
func (s *TorVMService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (ssec bool, errno uint32) {
	const acceptedCmds = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	engine := lifecycle.NewEngine(s.Config, s.Logger)

	// Notify SCM that the service is now running.
	changes <- svc.Status{State: svc.Running, Accepts: acceptedCmds}

	errCh := engine.Start(ctx)

	for {
		select {
		case err := <-errCh:
			if err != nil {
				s.Logger.Error("lifecycle error: %v", err)
				changes <- svc.Status{State: svc.StopPending}
				return false, 1
			}
			changes <- svc.Status{State: svc.StopPending}
			return false, 0

		case cr := <-r:
			switch cr.Cmd {
			case svc.Interrogate:
				changes <- cr.CurrentStatus
			case svc.Stop, svc.Shutdown:
				s.Logger.Info("received service %s request", cr.Cmd)
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				// Wait for engine to finish with a timeout.
				select {
				case <-errCh:
				case <-time.After(30 * time.Second):
					s.Logger.Error("engine shutdown timed out")
				}
				return false, 0
			}
		}
	}
}

// RunService runs the TorVM service under the Windows Service Control Manager.
// This should be called when the process is started with --service-run.
func RunService(cfg *config.Config, logger *logging.Logger) error {
	ew, err := NewEventLogWriter()
	if err != nil {
		logger.Error("failed to open event log: %v", err)
	} else {
		logger.AddWriter(ew)
		defer ew.Close()
	}

	svcHandler := &TorVMService{
		Config: cfg,
		Logger: logger,
	}

	if err := svc.Run(serviceName, svcHandler); err != nil {
		return fmt.Errorf("winsvc: service run failed: %w", err)
	}
	return nil
}

// InstallService registers TorVM as a Windows service.
func InstallService() error {
	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("winsvc: get executable path: %w", err)
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("winsvc: connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return fmt.Errorf("winsvc: service %q already exists", serviceName)
	}

	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceName,
		Description: serviceDescription,
		StartType:   mgr.StartAutomatic,
	}, "--service-run", "--headless")
	if err != nil {
		return fmt.Errorf("winsvc: create service: %w", err)
	}
	defer s.Close()

	// Set up the event log source for this service.
	if err := eventlog.InstallAsEventCreate(serviceName, eventlog.Error|eventlog.Warning|eventlog.Info); err != nil {
		// Non-fatal: service is installed but event log source may not work.
		s.Delete()
		return fmt.Errorf("winsvc: install event log source: %w", err)
	}

	return nil
}

// RemoveService unregisters the TorVM Windows service.
func RemoveService() error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("winsvc: connect to service manager: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err != nil {
		return fmt.Errorf("winsvc: open service: %w", err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("winsvc: delete service: %w", err)
	}

	if err := eventlog.Remove(serviceName); err != nil {
		// Non-fatal: service is removed but event log source may linger.
		return fmt.Errorf("winsvc: remove event log source: %w", err)
	}

	return nil
}
