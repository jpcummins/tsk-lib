package parse

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/jpcummins/tsk-lib/model"
	"github.com/jpcummins/tsk-lib/scan"
)

// rawConfig is the TOML structure for .config.toml files.
type rawConfig struct {
	Version   string              `toml:"version"`
	Defaults  rawDefaults         `toml:"defaults"`
	Inherit   rawInherit          `toml:"inherit"`
	Status    rawStatusSection    `toml:"status"`
	Iteration rawIterationSection `toml:"iteration"`
}

type rawDefaults struct {
	Assignee string `toml:"assignee"`
	Status   string `toml:"status"`
	Estimate string `toml:"estimate"`
}

type rawInherit struct {
	Assignee bool `toml:"assignee"`
	Status   bool `toml:"status"`
	Estimate bool `toml:"estimate"`
}

type rawStatusSection struct {
	Map map[string]rawStatusEntry `toml:"map"`
}

type rawIterationSection struct {
	Status rawStatusSection `toml:"status"`
}

type rawStatusEntry struct {
	Category string `toml:"category"`
	Order    int    `toml:"order"`
}

// rawTeamConfig is the TOML structure for team.toml files.
type rawTeamConfig struct {
	Members   []string            `toml:"members"`
	Iteration rawIterationSection `toml:"iteration"`
}

// rawSLAConfig is the TOML structure for sla.toml files.
type rawSLAConfig struct {
	Rule []rawSLARule `toml:"rule"`
}

type rawSLARule struct {
	ID       string `toml:"id"`
	Name     string `toml:"name"`
	Query    string `toml:"query"`
	Target   string `toml:"target"`
	Start    string `toml:"start"`
	Stop     string `toml:"stop"`
	Severity string `toml:"severity"`
}

// parseConfig converts a scanned config entry into a model.Config.
func parseConfig(entry scan.Entry) (*model.Config, error) {
	var raw rawConfig
	if err := toml.Unmarshal(entry.Content, &raw); err != nil {
		return nil, fmt.Errorf("parsing config %s: %w", entry.Path, err)
	}

	cfg := &model.Config{
		Path:    entry.Path,
		Version: raw.Version,
		Defaults: model.Defaults{
			Assignee: raw.Defaults.Assignee,
			Status:   raw.Defaults.Status,
			Estimate: raw.Defaults.Estimate,
		},
		Inherit: model.Inherit{
			Assignee: raw.Inherit.Assignee,
			Status:   raw.Inherit.Status,
			Estimate: raw.Inherit.Estimate,
		},
		StatusMap:          convertStatusMap(raw.Status.Map),
		IterationStatusMap: convertStatusMap(raw.Iteration.Status.Map),
	}

	return cfg, nil
}

// parseTeamConfig converts a scanned team.toml entry into a model.Team.
func parseTeamConfig(entry scan.Entry) (*model.Team, error) {
	var raw rawTeamConfig
	if err := toml.Unmarshal(entry.Content, &raw); err != nil {
		return nil, fmt.Errorf("parsing team config %s: %w", entry.Path, err)
	}

	// Derive team name from path: teams/<team>/team.toml
	parts := strings.Split(entry.Path, "/")
	teamName := ""
	if len(parts) >= 2 {
		teamName = parts[1]
	}

	team := &model.Team{
		Name:               teamName,
		IterationStatusMap: convertStatusMap(raw.Iteration.Status.Map),
	}

	// Parse members: "First Last <email@example.com>"
	for _, m := range raw.Members {
		member := parseTeamMember(m)
		team.Members = append(team.Members, member)
	}

	return team, nil
}

// memberEmailRe extracts email from "Name <email>" format.
var memberEmailRe = regexp.MustCompile(`<([^>]+)>`)

// parseTeamMember parses "First Last <email@example.com>" into a TeamMember.
func parseTeamMember(raw string) model.TeamMember {
	member := model.TeamMember{Display: raw}

	matches := memberEmailRe.FindStringSubmatch(raw)
	if len(matches) == 2 {
		member.Email = matches[1]
		member.Name = strings.TrimSpace(strings.Replace(raw, matches[0], "", 1))
	} else {
		member.Name = raw
	}

	return member
}

// parseSLARules converts sla.toml content into SLA rules.
func parseSLARules(entry scan.Entry) ([]*model.SLARule, error) {
	var raw rawSLAConfig
	if err := toml.Unmarshal(entry.Content, &raw); err != nil {
		return nil, fmt.Errorf("parsing sla.toml: %w", err)
	}

	var rules []*model.SLARule
	for _, r := range raw.Rule {
		target, err := model.ParseDuration(r.Target)
		if err != nil {
			return nil, fmt.Errorf("parsing SLA rule %q target: %w", r.ID, err)
		}
		rules = append(rules, &model.SLARule{
			ID:       r.ID,
			Name:     r.Name,
			Query:    r.Query,
			Target:   target,
			Start:    r.Start,
			Stop:     r.Stop,
			Severity: r.Severity,
		})
	}

	return rules, nil
}

// convertStatusMap converts raw TOML status entries into a model.StatusMap.
func convertStatusMap(raw map[string]rawStatusEntry) model.StatusMap {
	if len(raw) == 0 {
		return nil
	}
	sm := make(model.StatusMap, len(raw))
	for name, entry := range raw {
		sm[name] = model.StatusEntry{
			Category: model.StatusCategory(entry.Category),
			Order:    entry.Order,
		}
	}
	return sm
}
