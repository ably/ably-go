package ably

// This file isolates the import of "github.com/ably/vcdiff-go" to enable build-time optimization.
// The vcdiff-go dependency is only imported when NewVCDiffPlugin() is used, allowing applications
// that don't require VCDiff delta decoding to avoid including this dependency in their builds.

import (
	"github.com/ably/vcdiff-go"
)

// AblyVCDiffDecoder is a production-ready implementation of the VCDiffDecoder interface
// that uses the official ably/vcdiff-go library for delta decoding.
// Implements VCDiffDecoder interface for decoding VCDiff-encoded delta messages (PC3).
type AblyVCDiffDecoder struct{}

// Decode implements the VCDiffDecoder interface (VD2a, PC3a).
func (a AblyVCDiffDecoder) Decode(delta []byte, base []byte) ([]byte, error) {
	// Use the vcdiff-go library to decode the delta
	return vcdiff.Decode(base, delta)
}

// NewVCDiffPlugin creates a new AblyVCDiffDecoder plugin instance.
// Used publicly to set the VCDiff plugin in the client options.
//
// Example:
//
//	client, err := ably.NewREST(ably.WithVCDiffPlugin(ably.NewVCDiffPlugin()))
func NewVCDiffPlugin() VCDiffDecoder {
	return AblyVCDiffDecoder{}
}
