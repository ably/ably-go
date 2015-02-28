package ably

import (
	"log"
	"os"
	"strings"

	"github.com/ably/ably-go/config"
)

// Wrapper around config.Params
//
// This is only a convenience structure to be able to call ably.ClientOptions
// instead of importing Params from the config package.
type ClientOptions struct {
	config.Params

	ApiKey           string
	RealtimeEndpoint string
	RestEndpoint     string
	Logger           *log.Logger
}

func (c *ClientOptions) ToParams() config.Params {
	c.setLogger()

	if c.ApiKey != "" {
		c.parseApiKey()
	}

	c.Params.RestEndpoint = c.RestEndpoint
	c.Params.RealtimeEndpoint = c.RealtimeEndpoint

	return c.Params
}

func (c *ClientOptions) parseApiKey() {
	keyParts := strings.Split(c.ApiKey, ":")

	if len(keyParts) != 2 {
		c.AblyLogger.Error("ApiKey doesn't use the right format. Ignoring this parameter.")
		return
	}

	c.AppID = keyParts[0]
	c.AppSecret = keyParts[1]
}

func (c *ClientOptions) setLogger() {
	if c.Logger != nil {
		c.AblyLogger = &config.AblyLogger{c.Logger}
	}

	if c.AblyLogger == nil {
		c.AblyLogger = &config.AblyLogger{
			Logger: log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile),
		}
	}
}
