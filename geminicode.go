package main

import "time"

func NewGeminiCode() *Config {
	return &Config{
		Name:            "gemini",
		Bin:             "gemini",
		Args:            []string{"-y"},
		ReadyPattern:    `Gemini\s+CLI`,
		SendPrompt:      SendPromptSingleLine,
		GracePeriod:     10 * time.Second,
		FallbackTimeout: 10 * time.Second,
		ReadyWait:       2 * time.Second,
		FormatPrompt:    ClaudeFormatPrompt,
		IdlePatterns:    []string{`Type\s+your\s+message`},
	}
}
