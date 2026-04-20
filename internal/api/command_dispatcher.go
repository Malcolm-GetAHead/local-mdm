package api

import (
	"context"
	"log/slog"
	"sync"

	"github.com/malcolm-getahead/local-mdm/internal/models"
	"github.com/malcolm-getahead/local-mdm/internal/platform/macos"
	"github.com/malcolm-getahead/local-mdm/internal/repository"
)

// dispatchRequest is a unit of work for the async dispatcher.
type dispatchRequest struct {
	device *models.Device
	cmd    *models.DeviceCommand
}

// commandDispatcher processes command dispatch asynchronously via a worker pool.
type commandDispatcher struct {
	queue          chan dispatchRequest
	wg             sync.WaitGroup
	cmdRepo        repository.CommandRepository
	nanomdmService *macos.NanoMDMService
	logger         *slog.Logger
}

const (
	defaultWorkerCount = 4
	defaultQueueSize   = 256
)

func newCommandDispatcher(cmdRepo repository.CommandRepository, nanomdmService *macos.NanoMDMService, logger *slog.Logger) *commandDispatcher {
	return &commandDispatcher{
		queue:          make(chan dispatchRequest, defaultQueueSize),
		cmdRepo:        cmdRepo,
		nanomdmService: nanomdmService,
		logger:         logger,
	}
}

// Start launches worker goroutines.
func (d *commandDispatcher) Start() {
	for i := 0; i < defaultWorkerCount; i++ {
		d.wg.Add(1)
		go d.worker()
	}
	d.logger.Info("command dispatcher started", "workers", defaultWorkerCount, "queue_size", defaultQueueSize)
}

// Stop closes the queue and waits for workers to drain.
func (d *commandDispatcher) Stop() {
	close(d.queue)
	d.wg.Wait()
	d.logger.Info("command dispatcher stopped")
}

// Enqueue submits a command for async dispatch. Non-blocking — drops if queue is full.
func (d *commandDispatcher) Enqueue(device *models.Device, cmd *models.DeviceCommand) {
	select {
	case d.queue <- dispatchRequest{device: device, cmd: cmd}:
	default:
		d.logger.Warn("command dispatch queue full, command will be delivered on next sync",
			"device_id", device.ID, "command_type", cmd.CommandType)
	}
}

func (d *commandDispatcher) worker() {
	defer d.wg.Done()
	for req := range d.queue {
		d.dispatch(req.device, req.cmd)
	}
}

func (d *commandDispatcher) dispatch(device *models.Device, cmd *models.DeviceCommand) {
	ctx := context.Background()

	switch device.Platform {
	case models.PlatformMacOS:
		d.dispatchMacOS(ctx, device, cmd)
	case models.PlatformWindows:
		d.logger.Info("command queued for Windows OMA-DM sync",
			"device_id", device.ID, "command_type", cmd.CommandType)
	case models.PlatformAndroid:
		d.logger.Info("command queued for Android",
			"device_id", device.ID, "command_type", cmd.CommandType)
	}
}

func (d *commandDispatcher) dispatchMacOS(ctx context.Context, device *models.Device, cmd *models.DeviceCommand) {
	if d.nanomdmService == nil {
		return
	}

	var plist []byte
	switch cmd.CommandType {
	case models.CommandTypeLock:
		plist, _ = macos.BuildDeviceLockCommand("", "")
	case models.CommandTypeWipe:
		plist, _ = macos.BuildEraseDeviceCommand("")
	case models.CommandTypeRestart:
		plist, _ = macos.BuildRestartDeviceCommand()
	case models.CommandTypeInstallProfile:
		if profileData, ok := cmd.CommandData["raw_profile"].(string); ok {
			plist, _ = macos.BuildInstallProfileCommand([]byte(profileData))
		}
	case models.CommandTypeInstallApp:
		if id, ok := cmd.CommandData["identifier"].(string); ok {
			plist, _ = macos.BuildInstallApplicationCommand(0, id)
		}
	default:
		return
	}

	if len(plist) == 0 {
		return
	}

	if _, err := d.nanomdmService.SendCommand(ctx, device.DeviceID, plist); err != nil {
		d.logger.Error("failed to dispatch macOS command",
			"error", err, "device_id", device.ID, "command_type", cmd.CommandType)
		return
	}

	if err := d.cmdRepo.MarkSent(ctx, cmd.ID); err != nil {
		d.logger.Error("failed to mark command sent", "error", err, "cmd_id", cmd.ID)
	}
}
