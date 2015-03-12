package config

import (
	"log"
	"os"
	"strings"
)

type Params struct {
	RealtimeEndpoint string
	RestEndpoint     string

	ApiKey       string
	ClientID     string
	AppID        string
	AppSecret    string
	UseTokenAuth bool

	Protocol  string
	UseBinary bool
	Tls       bool

	AblyLogger *AblyLogger
	LogLevel   string
}

func (p *Params) Prepare() {
	p.setLogger()

	if p.ApiKey != "" {
		p.parseApiKey()
	}
}

func (p *Params) parseApiKey() {
	keyParts := strings.Split(p.ApiKey, ":")

	if len(keyParts) != 2 {
		p.AblyLogger.Error("ApiKey doesn't use the right format. Ignoring this parameter.")
		return
	}

	p.AppID = keyParts[0]
	p.AppSecret = keyParts[1]
}

func (p *Params) setLogger() {
	if p.AblyLogger == nil {
		p.AblyLogger = &AblyLogger{
			Logger: log.New(os.Stdout, "", log.Lmicroseconds|log.Lshortfile),
		}
	}
}
