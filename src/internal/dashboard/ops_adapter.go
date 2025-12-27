package dashboard

import "github.com/markus-barta/nixfleet/internal/ops"

// hubCommandSender adapts Hub to the ops.CommandSender interface.
type hubCommandSender struct {
	hub *Hub
}

// SendCommand implements ops.CommandSender.
func (h *hubCommandSender) SendCommand(hostID, command string) bool {
	return h.hub.SendCommand(hostID, command)
}

// GetOnlineHosts implements ops.CommandSender.
func (h *hubCommandSender) GetOnlineHosts() []string {
	return h.hub.GetOnlineHosts()
}

// Ensure hubCommandSender implements ops.CommandSender at compile time.
var _ ops.CommandSender = (*hubCommandSender)(nil)

// opExecutorWrapper adapts ops.Executor to opExecutorInterface.
// This wrapper handles the type conversion to avoid import cycles.
type opExecutorWrapper struct {
	exec *ops.Executor
}

// HandleCommandComplete implements opExecutorInterface.
func (w *opExecutorWrapper) HandleCommandComplete(hostID, opID string, exitCode int, output string) (interface{}, error) {
	return w.exec.HandleCommandComplete(hostID, opID, exitCode, output)
}

// Ensure opExecutorWrapper implements opExecutorInterface at compile time.
var _ opExecutorInterface = (*opExecutorWrapper)(nil)

