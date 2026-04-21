package main

import (
	"bytes"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

const doneMarker = "P0MX_DONE_SIGNAL"

type Config struct {
	Name            string
	Bin             string
	Args            []string
	ReadyPattern    string
	SendPrompt      func(ptmx *os.File, prompt string)
	FormatPrompt    func(prompt string) string
	IdlePatterns    []string
	GracePeriod     time.Duration
	FallbackTimeout time.Duration
	ReadyWait       time.Duration
}

func NewRegistry() map[string]*Config {
	return map[string]*Config{
		"opencode":   NewOpenCode(),
		"claudecode": NewClaudeCode(),
		"codex":      NewCodex(),
		"gemini":     NewGeminiCode(),
	}
}

func DefaultFormatPrompt(prompt string) string {
	return fmt.Sprintf(
		"%s\n\nIMPORTANT: After you have fully completed all the above tasks, you MUST print exactly this line on its own: %s. Do not skip this.",
		prompt, doneMarker,
	)
}

func ClaudeFormatPrompt(prompt string) string {
	return fmt.Sprintf(
		"%s. IMPORTANT: After fully completing all tasks, print exactly this on its own line: %s",
		prompt, doneMarker,
	)
}

func SendPromptTyped(ptmx *os.File, prompt string) {
	ptmx.Write([]byte{0x15})
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte{0x17})
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte(prompt))
	time.Sleep(100 * time.Millisecond)
	ptmx.Write([]byte{0x0d})
}

func SendPromptSingleLine(ptmx *os.File, prompt string) {
	singleLine := strings.ReplaceAll(prompt, "\n", " ")
	singleLine = strings.ReplaceAll(singleLine, "\r", " ")
	ptmx.Write([]byte(singleLine))
	time.Sleep(300 * time.Millisecond)
	ptmx.Write([]byte{0x0d})
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b\[.*?m`)

func StripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func JoinArgs(args []string) string {
	var buf bytes.Buffer
	for i, a := range args {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(a)
	}
	return buf.String()
}
