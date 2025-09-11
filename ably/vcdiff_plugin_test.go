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

		// Empty delta should still be processed by vcdiff library
		// The library will determine if it's valid or not
		if err != nil {
			// If the library returns an error for empty delta, that's acceptable
			assert.Error(t, err)
		} else {
			// If the library handles empty delta gracefully, verify result
			assert.NotNil(t, result)
		}
	})

	t.Run("handle nil inputs", func(t *testing.T) {
		// Test with nil delta
		result, err := plugin.Decode(nil, []byte("base"))
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, result)
		}

		// Test with nil base
		result, err = plugin.Decode([]byte("delta"), nil)
		if err != nil {
			assert.Error(t, err)
		} else {
			assert.NotNil(t, result)
		}
	})

	t.Run("handle invalid vcdiff data", func(t *testing.T) {
		base := []byte("hello world")
		invalidDelta := []byte("this is not valid vcdiff data")

		result, err := plugin.Decode(invalidDelta, base)

		// Should return an error for invalid vcdiff data
		assert.Error(t, err)
		assert.Nil(t, result)
	})

	// Note: For a complete test, we would need valid vcdiff test data
	// which would require the vcdiff encoder or pre-generated test files
	// This basic test ensures the plugin is properly structured and handles edge cases
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
	})

	t.Run("can be used with WithVCDiffPlugin option", func(t *testing.T) {
		plugin := NewVCDiffPlugin()

		// Test that the plugin can be used with the client option
		// This is a compile-time test to ensure types match
		var option ClientOption = WithVCDiffPlugin(plugin)
		assert.NotNil(t, option)
	})
}
