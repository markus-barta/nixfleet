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

