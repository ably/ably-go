package config

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

// This needs to use a timestamp in millisecond
// Use the previous function to generate them from a time.Time struct.
type ScopeParams struct {
	Start int64
	End   int64
	Unit  string
}

type TimestampMillisecond struct {
	time.Time
}

// Get a timestamp in millisecond
func NewTimestamp(t time.Time) int64 {
	tm := TimestampMillisecond{Time: t}
	return tm.ToInt()
}

func (t *TimestampMillisecond) ToInt() int64 {
	return int64(time.Nanosecond * time.Duration(t.UnixNano()) / time.Millisecond)
}

func (s *ScopeParams) EncodeValues(out *url.Values) error {
	if s.Start != 0 && s.End != 0 && s.Start > s.End {
		return fmt.Errorf("ScopeParams.Start can not be after ScopeParams.End")
	}

	if s.Start != 0 {
		out.Set("start", strconv.FormatInt(s.Start, 10))
	}

	if s.End != 0 {
		out.Set("end", strconv.FormatInt(s.End, 10))
	}

	if s.Unit != "" {
		out.Set("unit", s.Unit)
	}

	return nil
}
