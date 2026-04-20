package main

import "time"

func NewCodex() *Config {
	return &Config{
		Name:            "codex",
		Bin:             "codex",
		Args:            []string{"--no-alt-screen"},
		ReadyPattern:    `OpenAI\s+Codex|Run\s+/review\s+on\s+my\s+current\s+changes`,
		SendPrompt:      SendPromptTyped,
		GracePeriod:     10 * time.Second,
		FallbackTimeout: 8 * time.Second,
		ReadyWait:       1 * time.Second,
		FormatPrompt:    DefaultFormatPrompt,
		IdlePatterns:    []string{`Run\s+/review\s+on\s+my\s+current\s+changes`},
	}
}
