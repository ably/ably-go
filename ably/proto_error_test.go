package ably

import (
	"github.com/stretchr/testify/assert"
	"testing"
)

func Test_errorInfo_String(t *testing.T) {
	tests := []struct {
		name           string
		err            *errorInfo
		expectedString string
		expectedSize   int
	}{
		{
			name: "error with all fields",
			err: &errorInfo{
				StatusCode: 400,
				Code:       40012,
				HRef:       "ably.com/errors/40012",
				Message:    "something went wrong",
				Server:     "s1",
			},
			expectedString: "(msg=something went wrong,Code=40012,StatusCode=400,Href=ably.com/errors/40012,Server=s1)",
			expectedSize:   89,
		},
		{
			name: "error with no href",
			err: &errorInfo{
				StatusCode: 400,
				Code:       40012,
				Message:    "something went wrong",
				Server:     "s1",
			},
			expectedString: "(msg=something went wrong,Code=40012,StatusCode=400,Server=s1)",
			expectedSize:   62,
		},
		{
			name: "error with no server",
			err: &errorInfo{
				StatusCode: 400,
				Code:       40012,
				HRef:       "ably.com/errors/40012",
				Message:    "something went wrong",
			},
			expectedString: "(msg=something went wrong,Code=40012,StatusCode=400,Href=ably.com/errors/40012)",
			expectedSize:   79,
		},
		{
			name:           "nil error",
			err:            nil,
			expectedString: "(nil)",
			expectedSize:   5,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := tt.err.String()

			assert.Equal(t, tt.expectedString, s)
			assert.Equal(t, tt.expectedSize, len(s))
		})
	}
}
