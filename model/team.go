package model

// TeamMember represents a member of a team.
type TeamMember struct {
	// Identifier is the short handle (e.g., "alice").
	Identifier string

	// Value is the full value from team.toml (e.g., "Alice Smith <alice@example.com>").
	Value string

	// Name is the parsed display name (may be empty).
	Name string

	// Email is the parsed email address (may be empty).
	Email string
}

// Team represents a team defined under teams/<name>/.
type Team struct {
	// Name is the team identifier (directory name).
	Name string

	// Members maps member identifiers to their details.
	Members map[string]TeamMember
}
