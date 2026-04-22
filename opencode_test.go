package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestOpenCodeReadyPatternWaitsForPrompt(t *testing.T) {
	cfg := NewOpenCode()
	readyRe := regexp.MustCompile(cfg.ReadyPattern)

	startupBanner := StripANSI(`   ___    __    ________  ____________   ____
  /   |  / /   / ____/ / / / ____/ __ \ / __ \
 / /| | / /   / __/ / / / / __/ / /_/ // / / /
/ ___ |/ /___/ /___/ /_/ / /___/ __, // /_/ /
/_/  |_/_____/_____/\____/_____/____/ \___/_/   v1.0.0

Type 'help' to get started.
`)

	if readyRe.MatchString(startupBanner) {
		t.Fatal("OpenCode banner should not be treated as input-ready")
	}

	readyScreen := startupBanner + "\n\nAsk anything"
	if !readyRe.MatchString(readyScreen) {
		t.Fatal("OpenCode prompt should be treated as input-ready")
	}
}

func TestOpenCodeSendPromptTypedUsesControlChars(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	SendPromptTyped(writer, "my prompt")
	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	buf := make([]byte, 128)
	n, err := reader.Read(buf)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	got := string(buf[:n])
	if !strings.ContainsRune(got, '\x15') || !strings.ContainsRune(got, '\x17') {
		t.Fatal("typed prompt should send Ctrl+U (0x15) and Ctrl+W (0x17)")
	}
	if !strings.HasSuffix(got, "\r") {
		t.Fatalf("typed prompt should end with carriage return, got: %q", got)
	}
}
