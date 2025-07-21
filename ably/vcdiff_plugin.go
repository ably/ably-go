package ably

import (
	"github.com/ably/vcdiff-go"
)

// VCDiffPlugin is a production-ready implementation of the VCDiffDecoder interface
// that uses the official ably/vcdiff-go library for delta decoding.
type VCDiffPlugin struct{}

// NewVCDiffPlugin creates a new VCDiff plugin instance.
func NewVCDiffPlugin() *VCDiffPlugin {
	return &VCDiffPlugin{}
}

// Decode implements the VCDiffDecoder interface (VD2a, PC3a).
// It uses the vcdiff-go library to decode VCDiff delta data.
func (p *VCDiffPlugin) Decode(delta []byte, base []byte) ([]byte, error) {
	// Use the vcdiff-go library to decode the delta
	result, err := vcdiff.Decode(base, delta)
	if err != nil {
		return nil, err
	}
	
	return result, nil
}