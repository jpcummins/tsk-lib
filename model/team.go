package model

// Team represents a team directory under teams/ (Section 8).
type Team struct {
	Name    string
	Members []TeamMember

	// Iteration status mapping (from team.toml).
	IterationStatusMap StatusMap
}

// TeamMember represents a member entry from team.toml.
// Parsed from the "First Last <email@example.com>" format.
type TeamMember struct {
	Display string // Full display string (e.g., "Alice Smith <alice@example.com>").
	Name    string // Parsed name portion (e.g., "Alice Smith").
	Email   string // Parsed email (e.g., "alice@example.com").
}
