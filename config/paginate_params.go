package config

import (
	"fmt"
	"net/url"

	"github.com/ably/ably-go/Godeps/_workspace/src/github.com/google/go-querystring/query"
)

type PaginateParams struct {
	Limit     int    `url:"limit"`
	Direction string `url:"direction"`
}

func (p *PaginateParams) Values() (url.Values, error) {
	if p.Limit < 1 {
		p.Limit = 100
	}

	switch p.Direction {
	case "":
		p.Direction = "backwards"
		break
	case "backwards", "forwards":
		break
	default:
		return url.Values{}, fmt.Errorf("Invalid value for direction: %s", p.Direction)
	}

	return query.Values(p)
}
