//go:build windows

package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const windowsRunServiceCommand = "run-service"

func handlePlatformCommand(args []string) (bool, error) {
	if len(args) == 0 {
		return false, nil
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "service":
		return true, handleWindowsServiceCLI(args[1:])
	case windowsRunServiceCommand:
		return true, runWindowsServiceProcess(args[1:])
	default:
		return false, nil
	}
}

func platformServiceManager() string {
	return "windows-service"
}

func handleWindowsServiceCLI(args []string) error {
	if len(args) == 0 {
		return errors.New("missing service action")
	}

	switch strings.ToLower(strings.TrimSpace(args[0])) {
	case "install":
		return installWindowsService(args[1:])
	case "start":
		return startWindowsService(args[1:])
	case "stop":
		return stopWindowsService(args[1:])
	case "uninstall", "delete":
		return uninstallWindowsService(args[1:])
	default:
		return fmt.Errorf("unsupported service action: %s", args[0])
	}
}

func installWindowsService(args []string) error {
	fs := newFlagSet("service install")
	var configPath string
	var serviceName string
	fs.StringVar(&configPath, "config", "", "agent config path")
	fs.StringVar(&serviceName, "name", "", "windows service name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	configPath = strings.TrimSpace(configPath)
	serviceName = strings.TrimSpace(serviceName)
	if configPath == "" {
		return errors.New("missing --config")
	}
	if serviceName == "" {
		return errors.New("missing --name")
	}

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable path: %w", err)
	}
	absConfigPath, err := filepath.Abs(configPath)
	if err != nil {
		return fmt.Errorf("resolve config path: %w", err)
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	existing, err := manager.OpenService(serviceName)
	if err == nil {
		existing.Close()
		return fmt.Errorf("service already exists: %s", serviceName)
	}

	service, err := manager.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: serviceName,
		StartType:   mgr.StartAutomatic,
		Description: "OpsHub host agent",
	}, windowsRunServiceCommand, "--config", absConfigPath, "--service-name", serviceName)
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer service.Close()
	return nil
}

func startWindowsService(args []string) error {
	service, manager, err := openWindowsService(args, "service start")
	if err != nil {
		return err
	}
	defer service.Close()
	defer manager.Disconnect()

	status, err := service.Query()
	if err == nil && status.State == svc.Running {
		return nil
	}
	if err := service.Start(); err != nil {
		return fmt.Errorf("start service: %w", err)
	}
	return waitForWindowsServiceState(service, svc.Running, 30*time.Second)
}

func stopWindowsService(args []string) error {
	service, manager, err := openWindowsService(args, "service stop")
	if err != nil {
		return err
	}
	defer service.Close()
	defer manager.Disconnect()

	status, err := service.Query()
	if err == nil && status.State == svc.Stopped {
		return nil
	}
	if _, err := service.Control(svc.Stop); err != nil {
		return fmt.Errorf("stop service: %w", err)
	}
	return waitForWindowsServiceState(service, svc.Stopped, 30*time.Second)
}

func uninstallWindowsService(args []string) error {
	serviceName, err := parseWindowsServiceName(args, "service uninstall")
	if err != nil {
		return err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect service manager: %w", err)
	}
	defer manager.Disconnect()

	service, err := manager.OpenService(serviceName)
	if err != nil {
		return nil
	}
	defer service.Close()

	status, err := service.Query()
	if err == nil && status.State != svc.Stopped {
		_, _ = service.Control(svc.Stop)
		_ = waitForWindowsServiceState(service, svc.Stopped, 30*time.Second)
	}
	if err := service.Delete(); err != nil {
		return fmt.Errorf("delete service: %w", err)
	}
	return nil
}

func openWindowsService(args []string, name string) (*mgr.Service, *mgr.Mgr, error) {
	serviceName, err := parseWindowsServiceName(args, name)
	if err != nil {
		return nil, nil, err
	}

	manager, err := mgr.Connect()
	if err != nil {
		return nil, nil, fmt.Errorf("connect service manager: %w", err)
	}

	service, err := manager.OpenService(serviceName)
	if err != nil {
		manager.Disconnect()
		return nil, nil, fmt.Errorf("open service: %w", err)
	}
	return service, manager, nil
}

func parseWindowsServiceName(args []string, name string) (string, error) {
	fs := newFlagSet(name)
	var serviceName string
	fs.StringVar(&serviceName, "name", "", "windows service name")
	if err := fs.Parse(args); err != nil {
		return "", err
	}

	serviceName = strings.TrimSpace(serviceName)
	if serviceName == "" {
		return "", errors.New("missing --name")
	}
	return serviceName, nil
}

func waitForWindowsServiceState(service *mgr.Service, state svc.State, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		current, err := service.Query()
		if err != nil {
			return fmt.Errorf("query service status: %w", err)
		}
		if current.State == state {
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("service did not reach state %d within %s", state, timeout)
}

func runWindowsServiceProcess(args []string) error {
	fs := newFlagSet(windowsRunServiceCommand)
	var configPath string
	var serviceName string
	fs.StringVar(&configPath, "config", "", "agent config path")
	fs.StringVar(&serviceName, "service-name", "", "windows service name")
	if err := fs.Parse(args); err != nil {
		return err
	}

	configPath = strings.TrimSpace(configPath)
	serviceName = strings.TrimSpace(serviceName)
	if configPath == "" {
		return errors.New("missing --config")
	}
	if serviceName == "" {
		return errors.New("missing --service-name")
	}

	interactive, err := svc.IsAnInteractiveSession()
	if err == nil && interactive {
		return errors.New("run-service must be started by Windows SCM")
	}

	handler := &windowsAgentService{
		configPath:  configPath,
		serviceName: serviceName,
	}
	return svc.Run(serviceName, handler)
}

type windowsAgentService struct {
	configPath  string
	serviceName string
}

func (s *windowsAgentService) Execute(_ []string, requests <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	const acceptedCommands = svc.AcceptStop | svc.AcceptShutdown
	changes <- svc.Status{State: svc.StartPending}

	cfg, err := loadConfig(s.configPath)
	if err != nil {
		changes <- svc.Status{State: svc.Stopped}
		return true, 1
	}
	if strings.TrimSpace(cfg.ServiceName) == "" {
		cfg.ServiceName = s.serviceName
	}

	runCtx, cancel := context.WithCancel(context.Background())
	defer cancel()

	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- runAgent(runCtx, cfg)
	}()

	changes <- svc.Status{State: svc.Running, Accepts: acceptedCommands}

	for {
		select {
		case err := <-runErrCh:
			changes <- svc.Status{State: svc.StopPending}
			if err != nil {
				return false, 1
			}
			return false, 0
		case req := <-requests:
			switch req.Cmd {
			case svc.Interrogate:
				changes <- req.CurrentStatus
			case svc.Stop, svc.Shutdown:
				changes <- svc.Status{State: svc.StopPending}
				cancel()
				if err := <-runErrCh; err != nil {
					return false, 1
				}
				return false, 0
			default:
			}
		}
	}
}
