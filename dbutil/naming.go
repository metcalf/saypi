package dbutil

import (
	"bitbucket.org/pkg/inflect"
)

var (
	rs *inflect.Ruleset
	// Capitalized acronymns must be incorporated into the inflection ruleset.
	// An acronymns that is the substring of another acronymn should appear second.
	acronymns = []string{"SID", "ID", "URL"}
)

func init() {
	rs = inflect.NewDefaultRuleset()
	for _, acronymn := range acronymns {
		rs.AddAcronym(acronymn)
	}
}

func MapperFunc() func(string) string {
	return rs.Underscore
}
