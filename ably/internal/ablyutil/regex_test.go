//go:build !integration
// +build !integration

package ablyutil

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMatchDerivedChannel(t *testing.T) {
	var channels = []struct {
		name  string
		input string
		want  *derivedChannelMatch
	}{
		{"valid with base channel name", "foo", &derivedChannelMatch{QualifierParam: "", ChannelName: "foo"}},
		{"valid with base channel namespace", "foo:bar", &derivedChannelMatch{QualifierParam: "", ChannelName: "foo:bar"}},
		{"valid with existing qualifying option", "[?rewind=1]foo", &derivedChannelMatch{QualifierParam: "?rewind=1", ChannelName: "foo"}},
		{"fail with wrong channel option param", "[param=1]foo", nil},
		{"fail with invalid qualifying option", "[meta]foo", nil},
		{"fail with invalid regex match", "[failed-match]foo", nil},
	}

	for _, tt := range channels {
		t.Run(tt.name, func(t *testing.T) {
			match, err := MatchDerivedChannel(tt.input)
			if err != nil {
				assert.Error(t, err, "An error is expected for the regex match")
			}
			assert.Equal(t, tt.want, match, "invalid output received")
		})
	}
}
