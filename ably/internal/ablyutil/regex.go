package ablyutil

import (
	"errors"
	"fmt"
	"regexp"
)

// derivedChannelMatch provides the qualifyingParam and channelName from a channel regex match for derived channels
type derivedChannelMatch struct {
	QualifierParam string
	ChannelName    string
}

// This regex check is to retain existing channel params if any e.g [?rewind=1]foo to
// [filter=xyz?rewind=1]foo. This is to keep channel compatibility around use of
// channel params that work with derived channels.
func MatchDerivedChannel(name string) (*derivedChannelMatch, error) {
	regex := `^(\[([^?]*)(?:(.*))\])?(.+)$`
	r, err := regexp.Compile(regex)
	if err != nil {
		err := errors.New("regex compilation failed")
		return nil, err
	}
	match := r.FindStringSubmatch(name)

	if len(match) == 0 || len(match) < 5 {
		err := errors.New("regex match failed")
		return nil, err
	}
	// Fail if there is already a channel qualifier,
	// eg [meta]foo should fail instead of just overriding with [filter=xyz]foo
	if len(match[2]) > 0 {
		err := fmt.Errorf("cannot use a derived option with a %s channel", match[2])
		return nil, err
	}

	// Return match values to be added to derive channel quantifier.
	return &derivedChannelMatch{
		QualifierParam: match[3],
		ChannelName:    match[4],
	}, nil
}
