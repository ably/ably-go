package config

import (
	"net/url"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/google/go-querystring/query"
)

type PaginateParams struct {
	Limit     int    `url:"limit"`
	Direction string `url:"direction"`
}

func (p *PaginateParams) Values() (url.Values, error) {
	return query.Values(p)
}
