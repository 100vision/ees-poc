package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"

	"ees-demo/common/config"
	"ees-demo/common/log"
)

// agentService implements svc.Handler for the EES Agent Windows Service.
type agentService struct{}

// Execute is the service entry point called by svc.Run.
func (s *agentService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const cmdsAccepted = svc.AcceptStop | svc.AcceptShutdown

	changes <- svc.Status{State: svc.StartPending}

	// Load config (paths resolved relative to exe directory)
	cfg := config.DefaultConfig()
	logPath := resolvePath(cfg.LogPath)
	wlPath := resolvePath(cfg.Whitelist)

	logger, err := log.New(logPath, false)
	if err != nil {
		return true, 1
	}
	defer logger.Close()

	logger.Info("EES Agent service starting (PID=%d)", os.Getpid())
	logger.Info("Log path: %s", logPath)
	logger.Info("Whitelist path: %s", wlPath)

	// Start the named pipe server in a background goroutine.
	// Whitelist is hot-reloaded on each request, so we only pass the path.
	stopChan := make(chan struct{})
	doneChan := make(chan struct{})
	go func() {
		runPipeServer(cfg, wlPath, logger, stopChan)
		close(doneChan)
	}()

	// Service is now running
	changes <- svc.Status{State: svc.Running, Accepts: cmdsAccepted}
	logger.Info("EES Agent service started")

	// Wait for stop/shutdown commands from SCM
	for {
		select {
		case c := <-r:
			switch c.Cmd {
			case svc.Interrogate:
				changes <- c.CurrentStatus

			case svc.Stop, svc.Shutdown:
				logger.Info("EES Agent service stopping...")
				changes <- svc.Status{State: svc.StopPending}
				close(stopChan)
				<-doneChan
				logger.Info("EES Agent service stopped")
				return false, 0
			}
		}
	}
}

// installService creates and registers the Windows service.
func installService(name, displayName string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("get executable path: %w", err)
	}

	existing, err := m.OpenService(name)
	if err == nil {
		existing.Close()
		return fmt.Errorf("service %q already exists (use uninstall first)", name)
	}

	svc, err := m.CreateService(
		name,
		exePath,
		mgr.Config{
			DisplayName: displayName,
			Description: "Verifies and elevates enterprise-approved software installers for standard users.",
			StartType:   mgr.StartAutomatic,
		},
	)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer svc.Close()

	return nil
}

// uninstallService removes the Windows service.
func uninstallService(name string) error {
	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(name)
	if err != nil {
		return fmt.Errorf("open service %q: %w", name, err)
	}
	defer s.Close()

	if err := s.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

// runAgent runs the agent logic as a console application (debug mode).
func runAgent(logger *log.Logger) {
	if logger == nil {
		logger = log.NewConsole()
	}

	cfg := config.DefaultConfig()
	logPath := resolvePath(cfg.LogPath)
	wlPath := resolvePath(cfg.Whitelist)

	logger.Info("EES Agent debug mode (PID=%d)", os.Getpid())
	logger.Info("Pipe: %s", cfg.PipeName)
	logger.Info("Log path: %s", logPath)
	logger.Info("Whitelist path: %s", wlPath)
	logger.Info("Whitelist hot-reload enabled — edit whitelist.json without restarting")

	stopChan := make(chan struct{})

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info("Shutdown requested...")
		close(stopChan)
	}()

	runPipeServer(cfg, wlPath, logger, stopChan)
	logger.Info("Agent stopped")
}
