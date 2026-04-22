package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestGeminiCodeReadyPatternWaitsForPrompt(t *testing.T) {
	cfg := NewGeminiCode()
	readyRe := regexp.MustCompile(cfg.ReadyPattern)

	startupBanner := StripANSI(`   🔷  Google AI    Gemini   v2.0.1

  ━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Welcome! Type your message to begin.

`)

	if readyRe.MatchString(startupBanner) {
		t.Fatal("Gemini Code banner should not be treated as input-ready")
	}

	readyScreen := startupBanner + "\nGemini CLI ready"
	if !readyRe.MatchString(readyScreen) {
		t.Fatal("Gemini Code prompt should be treated as input-ready")
	}
}

func TestGeminiCodeSendPromptSingleLineAvoidsControlChars(t *testing.T) {
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	defer reader.Close()
	defer writer.Close()

	SendPromptSingleLine(writer, "hello world\nfoo bar")
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
	if got != "hello world foo bar\r" {
		t.Fatalf("unexpected prompt bytes: %q", got)
	}
}

func TestGeminiCodeHasArgs(t *testing.T) {
	cfg := NewGeminiCode()
	if len(cfg.Args) == 0 {
		t.Fatal("Gemini Code should have args (e.g., -y for auto-confirm)")
	}
	if cfg.Args[0] != "-y" {
		t.Fatalf("expected first arg to be -y, got: %v", cfg.Args)
	}
}
