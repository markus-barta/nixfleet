// Package agent implements the NixFleet agent.
// This file implements P2810: 3-layer binary freshness detection.
package agent

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/markus-barta/nixfleet/internal/protocol"
)

// SourceCommit is set at build time via ldflags.
// go build -ldflags "-X github.com/markus-barta/nixfleet/internal/agent.SourceCommit=$(git rev-parse HEAD)"
var SourceCommit = "unknown"

// Freshness holds the computed binary freshness data.
type Freshness struct {
	SourceCommit string // From ldflags at build time
	StorePath    string // Nix store path of running binary
	BinaryHash   string // SHA256 of binary
}

// cachedFreshness stores computed freshness (computed once on startup)
var cachedFreshness *Freshness

// GetFreshness returns the agent's binary freshness data.
// Computed once and cached since it doesn't change during runtime.
func GetFreshness() *Freshness {
	if cachedFreshness != nil {
		return cachedFreshness
	}

	cachedFreshness = &Freshness{
		SourceCommit: SourceCommit,
		StorePath:    getStorePath(),
		BinaryHash:   computeBinaryHash(),
	}

	return cachedFreshness
}

// ToProtocol converts to protocol AgentFreshness type.
func (f *Freshness) ToProtocol() protocol.AgentFreshness {
	return protocol.AgentFreshness{
		SourceCommit: f.SourceCommit,
		StorePath:    f.StorePath,
		BinaryHash:   f.BinaryHash,
	}
}

// getStorePath returns the Nix store path of the running binary.
func getStorePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	// On Linux, resolve symlinks to get actual path
	if runtime.GOOS != "darwin" {
		// Try to read /proc/self/exe for accurate path
		realPath, err := os.Readlink("/proc/self/exe")
		if err == nil {
			exePath = realPath
		}
	}

	// Check if it's a Nix store path
	if strings.HasPrefix(exePath, "/nix/store/") {
		return exePath
	}

	// Return whatever path we got
	return exePath
}

// computeBinaryHash computes SHA256 hash of the running binary.
func computeBinaryHash() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}

	// On Linux, resolve to actual path
	if runtime.GOOS != "darwin" {
		realPath, err := os.Readlink("/proc/self/exe")
		if err == nil {
			exePath = realPath
		}
	}

	f, err := os.Open(exePath)
	if err != nil {
		return ""
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return ""
	}

	return hex.EncodeToString(h.Sum(nil))
}

