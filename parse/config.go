package parse

import (
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jpcummins/tsk-lib/model"
)

// rawConfig is the TOML representation of a config.toml file.
type rawConfig struct {
	Version string `toml:"version"`
}

// rawTeamConfig is the TOML representation of a team.toml file.
type rawTeamConfig struct {
	Members map[string]string `toml:"members"`
}

// rawSLAFile is the TOML representation of an sla.toml file.
type rawSLAFile struct {
	Rule []rawSLARule `toml:"rule"`
}

// rawSLARule is the TOML representation of an SLA rule.
type rawSLARule struct {
	ID       string `toml:"id"`
	Name     string `toml:"name"`
	Query    string `toml:"query"`
	Target   string `toml:"target"`
	WarnAt   string `toml:"warn_at"`
	Start    string `toml:"start"`
	Stop     string `toml:"stop"`
	Severity string `toml:"severity"`
}

// parseConfig parses a config.toml file.
func parseConfig(data []byte, path string) (*model.Config, error) {
	var raw rawConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	cfg := &model.Config{
		Path: model.CanonicalPath(path),
	}

	// Version is only valid at root
	isRoot := path == "" || path == "config.toml"
	if isRoot {
		cfg.Version = raw.Version
	}

	return cfg, nil
}

// parseTeamConfig parses a team.toml file.
func parseTeamConfig(data []byte, teamName string) (*model.Team, error) {
	var raw rawTeamConfig
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	team := &model.Team{
		Name:    teamName,
		Members: make(map[string]model.TeamMember, len(raw.Members)),
	}

	for id, value := range raw.Members {
		member := model.TeamMember{
			Identifier: id,
			Value:      value,
		}

		// Parse "First Last <email>" format
		name, email := parseMemberValue(value)
		member.Name = name
		member.Email = email

		team.Members[id] = member
	}

	return team, nil
}

// parseMemberValue parses member value formats:
// - "First Last <email@example.com>"
// - "First Last"
// - "email@example.com"
func parseMemberValue(value string) (name, email string) {
	value = strings.TrimSpace(value)

	// Check for "Name <email>" format
	if idx := strings.Index(value, "<"); idx >= 0 {
		name = strings.TrimSpace(value[:idx])
		end := strings.Index(value, ">")
		if end > idx {
			email = value[idx+1 : end]
		}
		return
	}

	// Check if it's an email
	if strings.Contains(value, "@") {
		email = value
		return
	}

	// Otherwise it's just a name
	name = value
	return
}

// parseSLAFile parses an sla.toml file.
func parseSLAFile(data []byte) ([]*model.SLARule, error) {
	var raw rawSLAFile
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	rules := make([]*model.SLARule, 0, len(raw.Rule))
	for _, r := range raw.Rule {
		target, err := model.ParseDuration(r.Target)
		if err != nil {
			return nil, err
		}

		rule := &model.SLARule{
			ID:       r.ID,
			Name:     r.Name,
			Query:    r.Query,
			Target:   target,
			Start:    r.Start,
			Stop:     r.Stop,
			Severity: r.Severity,
		}

		if r.WarnAt != "" {
			warnAt, err := model.ParseDuration(r.WarnAt)
			if err != nil {
				return nil, err
			}
			rule.WarnAt = &warnAt
		}

		rules = append(rules, rule)
	}

	return rules, nil
}
