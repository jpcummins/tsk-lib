package parse

import (
	"path"
	"sort"
	"strings"

	"github.com/jp/tsk-lib/model"
)

// configChain builds the ordered list of configs from root to most specific
// for a given task path. Used to resolve defaults and status maps.
func configChain(taskPath model.CanonicalPath, configs []*model.Config) []*model.Config {
	var chain []*model.Config

	for _, cfg := range configs {
		scope := configScope(cfg.Path)
		if scope == "" || taskPath.HasPrefix(model.CanonicalPath(scope)) {
			chain = append(chain, cfg)
		}
	}

	// Sort by specificity: root (shortest scope) first, most specific last.
	sort.Slice(chain, func(i, j int) bool {
		return len(configScope(chain[i].Path)) < len(configScope(chain[j].Path))
	})

	return chain
}

// configScope computes the canonical scope a config applies to.
// Root .config.toml -> "" (applies to everything).
// tasks/shopping-cart/backend/.config.toml -> "shopping-cart/backend".
func configScope(cfgPath string) string {
	// Root config.
	if cfgPath == ".config.toml" {
		return ""
	}

	// Strip "tasks/" prefix and "/.config.toml" suffix.
	scope := strings.TrimPrefix(cfgPath, "tasks/")
	scope = strings.TrimSuffix(scope, "/.config.toml")

	return strings.ToLower(scope)
}

// resolveInheritance applies config defaults and inheritance to tasks.
// It also resolves custom status -> base category mapping.
func resolveInheritance(
	tasks map[model.CanonicalPath]*model.Task,
	iterations []*model.Iteration,
	configs []*model.Config,
	teams []*model.Team,
) {
	// Build merged status maps for each config scope.
	for _, task := range tasks {
		chain := configChain(task.CanonicalPath, configs)
		statusMap := mergeStatusMaps(chain)

		// Apply defaults from the inheritance chain.
		applyDefaults(task, chain)

		// Resolve custom status to base category.
		if task.Status != "" {
			task.StatusCategory = statusMap.Resolve(task.Status)
		}
	}

	// Resolve iteration status categories using team-specific maps.
	teamMap := make(map[string]*model.Team)
	for _, t := range teams {
		teamMap[t.Name] = t
	}

	for _, iter := range iterations {
		statusMap := iterationStatusMap(iter.Team, configs, teamMap)
		if iter.Status != "" {
			iter.StatusCategory = statusMap.Resolve(iter.Status)
		}
	}
}

// applyDefaults applies inherited default values from the config chain.
// Nearest config wins; task front matter overrides everything.
func applyDefaults(task *model.Task, chain []*model.Config) {
	// Walk the chain from most specific to least specific.
	// We only apply a default if the field is empty AND inheritance is enabled.
	for i := len(chain) - 1; i >= 0; i-- {
		cfg := chain[i]

		if cfg.Inherit.Assignee && task.Assignee == "" && cfg.Defaults.Assignee != "" {
			task.Assignee = cfg.Defaults.Assignee
		}
		if cfg.Inherit.Status && task.Status == "" && cfg.Defaults.Status != "" {
			task.Status = cfg.Defaults.Status
		}
		if cfg.Inherit.Estimate && task.Estimate == nil && cfg.Defaults.Estimate != "" {
			dur, err := model.ParseDuration(cfg.Defaults.Estimate)
			if err == nil {
				task.Estimate = &dur
			}
		}
	}
}

// mergeStatusMaps merges status maps from a config chain.
// More specific configs override less specific ones.
func mergeStatusMaps(chain []*model.Config) model.StatusMap {
	merged := make(model.StatusMap)

	for _, cfg := range chain {
		for k, v := range cfg.StatusMap {
			merged[k] = v
		}
	}

	return merged
}

// iterationStatusMap builds the effective status map for an iteration.
// Team-level iteration status map takes precedence over project-level.
func iterationStatusMap(
	teamName string,
	configs []*model.Config,
	teams map[string]*model.Team,
) model.StatusMap {
	// Start with any iteration status maps from project configs.
	merged := make(model.StatusMap)
	for _, cfg := range configs {
		for k, v := range cfg.IterationStatusMap {
			merged[k] = v
		}
	}

	// Team-specific iteration status map overrides.
	if team, ok := teams[teamName]; ok {
		for k, v := range team.IterationStatusMap {
			merged[k] = v
		}
	}

	return merged
}

// taskDirPath returns the directory scope for a task path.
// For a task at "shopping-cart/backend/cart-service-endpoints",
// this returns "shopping-cart/backend".
func taskDirPath(taskPath model.CanonicalPath) string {
	return path.Dir(string(taskPath))
}
