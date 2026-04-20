package main

import "time"

func NewOpenCode() *Config {
	return &Config{
		Name:            "opencode",
		Bin:             "opencode",
		ReadyPattern:    `Ask\s+anything`,
		SendPrompt:      SendPromptTyped,
		GracePeriod:     8 * time.Second,
		FallbackTimeout: 5 * time.Second,
		ReadyWait:       800 * time.Millisecond,
		FormatPrompt:    DefaultFormatPrompt,
		IdlePatterns:    []string{`Ask\s+anything`},
	}
}
