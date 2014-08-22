package ably

import "encoding/json"

type Params struct {
	RestEndpoint     string
	RealtimeEndpoint string
	AppID            string
	AppSecret        string
	ClientID         string
}

type Capability map[string][]string

func (c *Capability) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(b)
}
