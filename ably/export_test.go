package ably

func Wait(res Result, err error) error {
	return wait(res, err)
}

func (p *PaginatedResult) BuildPath(base, rel string) (string, error) {
	return p.buildPath(base, rel)
}

func (opts *ClientOptions) RestURL() string {
	return opts.restURL()
}

func (opts *ClientOptions) RealtimeURL() string {
	return opts.realtimeURL()
}
