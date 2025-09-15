package ably

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVCDiffPlugin(t *testing.T) {
	plugin := NewVCDiffPlugin()
	require.NotNil(t, plugin)

	t.Run("implements VCDiffDecoder interface", func(t *testing.T) {
		var _ VCDiffDecoder = plugin
	})

	t.Run("returns AblyVCDiffDecoder type", func(t *testing.T) {
		_, ok := plugin.(AblyVCDiffDecoder)
		assert.True(t, ok, "NewVCDiffPlugin should return AblyVCDiffDecoder type")
	})

	t.Run("handle empty delta", func(t *testing.T) {
		base := []byte("hello world")
		delta := []byte{}

		result, err := plugin.Decode(delta, base)

		// Empty delta should return an error as it's not valid vcdiff data
		assert.Error(t, err, "Empty delta should be invalid vcdiff data")
		assert.Nil(t, result, "Result should be nil when decode fails")
	})

	t.Run("handle nil inputs", func(t *testing.T) {
		// Test with nil delta - should return error
		result, err := plugin.Decode(nil, []byte("base"))
		assert.Error(t, err, "Nil delta should cause decode error")
		assert.Nil(t, result, "Result should be nil when decode fails")

		// Test with nil base - should return error
		result, err = plugin.Decode([]byte("delta"), nil)
		assert.Error(t, err, "Nil base should cause decode error")
		assert.Nil(t, result, "Result should be nil when decode fails")

		// Test with both nil - should return error
		result, err = plugin.Decode(nil, nil)
		assert.Error(t, err, "Both nil should cause decode error")
		assert.Nil(t, result, "Result should be nil when decode fails")
	})

	t.Run("handle invalid vcdiff data", func(t *testing.T) {
		base := []byte("hello world")
		invalidDelta := []byte("this is not valid vcdiff data")

		result, err := plugin.Decode(invalidDelta, base)

		// Should return an error for invalid vcdiff data
		assert.Error(t, err, "Invalid vcdiff data should cause decode error")
		assert.Nil(t, result, "Result should be nil when decode fails")
	})

	t.Run("decode with empty base", func(t *testing.T) {
		// Test behavior with empty base payload
		emptyBase := []byte{}
		invalidDelta := []byte("some delta data")

		result, err := plugin.Decode(invalidDelta, emptyBase)

		// Should return an error since this isn't valid vcdiff usage
		assert.Error(t, err, "Decode with empty base should cause error")
		assert.Nil(t, result, "Result should be nil when decode fails")
	})

	// Note: For complete testing with valid vcdiff data, we would need:
	// 1. A vcdiff encoder to create test delta data, OR
	// 2. Pre-generated valid vcdiff test files with known base/delta/target
	// The current tests verify error handling and type safety which are critical
	// for the plugin system integration.
}

func TestVCDiffPlugin_Integration(t *testing.T) {
	t.Run("can be used as VCDiffDecoder in DecodingContext", func(t *testing.T) {
		plugin := NewVCDiffPlugin()

		context := &DecodingContext{
			VCDiffPlugin:  plugin,
			BasePayload:   []byte("base data"),
			LastMessageID: "msg-123",
		}

		assert.NotNil(t, context.VCDiffPlugin)
		assert.Equal(t, plugin, context.VCDiffPlugin)

		// Verify the plugin can be called through the context
		// (This will fail since we don't have valid vcdiff data, but tests the interface)
		_, err := context.VCDiffPlugin.Decode([]byte("invalid"), context.BasePayload)
		assert.Error(t, err, "Expected error with invalid vcdiff data")
	})

	t.Run("can be used with WithVCDiffPlugin option", func(t *testing.T) {
		plugin := NewVCDiffPlugin()

		// Test that the plugin can be used with the client option
		// This is a compile-time test to ensure types match
		var option ClientOption = WithVCDiffPlugin(plugin)
		assert.NotNil(t, option)

		// Test that the option actually sets the plugin in client options
		opts := &clientOptions{}
		option(opts)
		assert.Equal(t, plugin, opts.VCDiffPlugin, "Plugin should be set in client options")
	})

	t.Run("plugin is reusable across multiple decodes", func(t *testing.T) {
		plugin := NewVCDiffPlugin()

		// Test that the same plugin instance can be used multiple times
		// Even though these will fail, it tests that the plugin doesn't maintain state
		for i := 0; i < 3; i++ {
			result, err := plugin.Decode([]byte("invalid"), []byte("base"))
			assert.Error(t, err, "Each decode should fail with invalid data")
			assert.Nil(t, result, "Each result should be nil")
		}
	})
}
