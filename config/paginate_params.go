package config

import (
	"fmt"
	"net/url"
	"strconv"
)

type PaginateParams struct {
	Limit     int
	Direction string

	ScopeParams
}

func (p *PaginateParams) EncodeValues(out *url.Values) error {
	if p.Limit < 0 {
		out.Set("limit", strconv.Itoa(100))
	} else if p.Limit != 0 {
		out.Set("limit", strconv.Itoa(p.Limit))
	}

	switch p.Direction {
	case "":
		break
	case "backwards", "forwards":
		out.Set("direction", p.Direction)
		break
	default:
		return fmt.Errorf("Invalid value for direction: %s", p.Direction)
	}

	p.ScopeParams.EncodeValues(out)

	return nil
}
