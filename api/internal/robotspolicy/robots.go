package robotspolicy

import (
	"regexp"
	"strings"
)

type robotRule struct {
	allow   bool
	pattern string
}

func ruleMatches(pattern, path string) bool {
	if pattern == "" {
		return false
	}
	re := "^" + regexp.QuoteMeta(pattern)
	re = strings.ReplaceAll(re, `\*`, ".*")
	if strings.HasSuffix(re, `\$`) {
		re = strings.TrimSuffix(re, `\$`) + "$"
	}
	compiled, e := regexp.Compile(re)
	return e == nil && compiled.MatchString(path)
}
func Allowed(content, path string) bool {
	type group struct {
		agents []string
		rules  []robotRule
	}
	groups := []group{}
	current := group{}
	appendGroup := func() {
		if len(current.agents) > 0 {
			groups = append(groups, current)
		}
		current = group{}
	}
	for _, line := range strings.Split(content, "\n") {
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		p := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(p) != 2 {
			continue
		}
		key, value := strings.ToLower(strings.TrimSpace(p[0])), strings.TrimSpace(p[1])
		switch key {
		case "user-agent":
			if len(current.rules) > 0 {
				appendGroup()
			}
			current.agents = append(current.agents, strings.ToLower(value))
		case "allow", "disallow":
			if len(current.agents) > 0 {
				current.rules = append(current.rules, robotRule{allow: key == "allow", pattern: value})
			}
		}
	}
	appendGroup()
	// Specific StoreLift rules override wildcard groups; in a matching group
	// the most specific path wins, and Allow wins ties.
	bestAgent := 0
	rules := []robotRule{}
	for _, g := range groups {
		strength := 0
		for _, a := range g.agents {
			switch a {
			case "storeliftauditbot", "storelift":
				strength = 2
			case "*":
				if strength == 0 {
					strength = 1
				}
			}
		}
		if strength > bestAgent {
			bestAgent = strength
			rules = rules[:0]
		}
		if strength > 0 && strength == bestAgent {
			rules = append(rules, g.rules...)
		}
	}
	allowed := true
	longest := -1
	for _, rule := range rules {
		if !ruleMatches(rule.pattern, path) {
			continue
		}
		size := len(strings.TrimSuffix(strings.ReplaceAll(rule.pattern, "*", ""), "$"))
		if size > longest || (size == longest && rule.allow) {
			allowed = rule.allow
			longest = size
		}
	}
	return allowed
}
