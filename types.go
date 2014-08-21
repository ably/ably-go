package ably

import "encoding/json"

type Message struct {
	Name string `json:"name"`
	Data string `json:"data"`
}

type Capability map[string][]string

func (c *Capability) String() string {
	b, err := json.Marshal(c)
	if err != nil {
		panic(err)
	}
	return string(b)
}
