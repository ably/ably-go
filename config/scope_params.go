package config

import "time"

type TimestampMillisecond struct {
	time.Time
}

func (t *TimestampMillisecond) ToInt() int64 {
	return int64(time.Nanosecond * time.Duration(t.UnixNano()) / time.Millisecond)
}

// Get a timestamp in millisecond
func NewTimestamp(t time.Time) int64 {
	tm := TimestampMillisecond{Time: t}
	return tm.ToInt()
}

// This needs to use a timestamp in millisecond
// Use the previous function to generate them from a time.Time struct.
type ScopeParams struct {
	Start int64 `url:"start"`
	End   int64 `url:"end"`
}
