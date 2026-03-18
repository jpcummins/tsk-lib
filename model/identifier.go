package model

import "regexp"

// identifierRe matches valid identifiers per spec 1.2.4:
// lowercase alphanumeric with hyphens, must start with alphanumeric.
var identifierRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// ValidateIdentifier checks if a string is a valid identifier.
// Valid identifiers match [a-z0-9][a-z0-9-]*.
func ValidateIdentifier(s string) bool {
	if s == "" {
		return false
	}
	return identifierRe.MatchString(s)
}
