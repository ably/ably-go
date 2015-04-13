package ably

import "log"

type AblyLogger struct {
	*log.Logger
}

func (a *AblyLogger) Error(v ...interface{}) {
	a.Logger.Print("ERRO: "+v[0].(string), v[1:])
}

func (a *AblyLogger) Warn(v ...interface{}) {
	a.Logger.Print("WARN: "+v[0].(string), v[1:])
}

func (a *AblyLogger) Info(v ...interface{}) {
	a.Logger.Print("INFO: "+v[0].(string), v[1:])
}
