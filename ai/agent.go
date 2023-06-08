package ai

import (
	"strings"
)

type Provider string

var (
	DefaultAgent = "chatgpt"

	// alias to provider mapping
	Agents = map[string]Provider{
		"chatgpt": "openai",
	}
)

// IsPrompt checks whether we were prompted with @alias
func IsPrompt(p string, users int) (string, bool) {
	p = strings.ToLower(p)

	for agent := range Agents {
		if strings.Contains(p, "@"+agent) {
			return agent, true
		}
	}

	// 1:1 chat
	return DefaultAgent, users <= 1
}
