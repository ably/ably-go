package config

type Params struct {
	RealtimeEndpoint string
	RestEndpoint     string
	AppID            string
	AppSecret        string
	ClientID         string

	AblyLogger *AblyLogger
}
