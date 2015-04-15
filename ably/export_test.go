package ably

var BuildPath = func(p *PaginatedResource, base, rel string) (string, error) {
	return p.buildPath(base, rel)
}
