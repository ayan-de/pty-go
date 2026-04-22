package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestClaudeCodeReadyPatternWaitsForPrompt(t *testing.T) {
	cfg := NewClaudeCode()
	readyRe := regexp.MustCompile(cfg.ReadyPattern)

	startupBanner := StripANSI(`╭───────────────────────────────────────╮
│ >_ Claude Code (desktop)            │
│                                       │
│ Level: 5 (Pro)                        │
│ Token usage: 12,345 / 200,000         │
╰───────────────────────────────────────╯

  Tip: Press Ctrl+C to exit. Type "clear" to clear the screen.
`)

	if readyRe.MatchString(startupBanner) {
		t.Fatal("Claude Code banner should not be treated as input-ready")
	}

	readyScreen := startupBanner + "\n\n\n\nPress Ctrl-C again to exit"
	if !readyRe.MatchString(readyScreen) {
		t.Fatal("Claude Code prompt should be treated as input-ready")
	}
}

func TestClaudeCodeSendPromptSingleLineAvoidsControlChars(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	SendPromptSingleLine(writer, "first line\nsecond line")
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	buf := make([]byte, 128)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	got := string(buf[:n])
	if strings.ContainsRune(got, '\x15') || strings.ContainsRune(got, '\x17') {
		t.Fatal("single-line prompt should not send readline control characters")
	}
	if got != "first line second line\r" {
		t.Fatalf("unexpected prompt bytes: %q", got)
	}
}
