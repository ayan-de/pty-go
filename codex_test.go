package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestCodexReadyPatternWaitsForPrompt(t *testing.T) {
	cfg := NewCodex()
	readyRe := regexp.MustCompile(cfg.ReadyPattern)

	startupBanner := StripANSI(`╭───────────────────────────────────────╮
│ >_ OpenAI Codex (v0.122.0)            │
│                                       │
│ model:     gpt-5.4   /model to change │
│ directory: ~/Project/pty-go           │
╰───────────────────────────────────────╯

  Tip: New For a limited time, Codex is included in your plan for free – let’s build together.
`)

	if readyRe.MatchString(startupBanner) {
		t.Fatal("Codex banner should not be treated as input-ready")
	}

	readyScreen := startupBanner + "\n\n› "
	if !readyRe.MatchString(readyScreen) {
		t.Fatal("Codex prompt should be treated as input-ready")
	}
}

func TestSendPromptSingleLineAvoidsControlChars(t *testing.T) {
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
