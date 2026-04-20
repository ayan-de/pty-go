package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

const doneMarker = "P0MX_DONE_SIGNAL"

type state int

const (
	stateWaitingReady state = iota
	stateSendingPrompt
	stateWorking
	stateDone
)

type agent struct {
	name            string
	bin             string
	readyPattern    string
	sendPromptFn    func(ptmx *os.File, prompt string)
	formatterFn     func(prompt string) string
	idlePatterns    []string
	gracePeriod     time.Duration
	fallbackTimeout time.Duration
	readyWait       time.Duration
}

var agents = map[string]agent{
	"opencode": {
		name:            "opencode",
		bin:             "opencode",
		readyPattern:    `Ask\s+anything`,
		sendPromptFn:    sendPromptTyped,
		gracePeriod:     8 * time.Second,
		fallbackTimeout: 5 * time.Second,
		readyWait:       800 * time.Millisecond,
		formatterFn: func(prompt string) string {
			return fmt.Sprintf(
				"%s\n\nIMPORTANT: After you have fully completed all the above tasks, you MUST print exactly this line on its own: %s. Do not skip this.",
				prompt, doneMarker,
			)
		},
		idlePatterns: []string{`Ask\s+anything`},
	},
	"claudecode": {
		name:            "claude-code",
		bin:             "claude",
		readyPattern:    `Press\s+Ctrl-C\s+again\s+to\s+exit`,
		sendPromptFn:    sendPromptForClaude,
		gracePeriod:     10 * time.Second,
		fallbackTimeout: 10 * time.Second,
		readyWait:       2 * time.Second,
		formatterFn: func(prompt string) string {
			return fmt.Sprintf(
				"%s. IMPORTANT: After fully completing all tasks, print exactly this on its own line: %s",
				prompt, doneMarker,
			)
		},
		idlePatterns: []string{`Press\s+Ctrl-C\s+again\s+to\s+exit`},
	},
}

func main() {
	var chdir string
	var autoExit bool
	var agentName string
	var args []string
	for i := 1; i < len(os.Args); i++ {
		switch os.Args[i] {
		case "-chdir":
			i++
			if i < len(os.Args) {
				chdir = os.Args[i]
			}
		case "-auto-exit":
			autoExit = true
		case "-opencode":
			agentName = "opencode"
		case "-claudecode", "-claude":
			agentName = "claudecode"
		default:
			args = append(args, os.Args[i])
		}
	}

	if agentName == "" {
		agentName = "opencode"
	}

	ag, ok := agents[agentName]
	if !ok {
		fmt.Fprintf(os.Stderr, "unknown agent: %s\n", agentName)
		os.Exit(1)
	}

	prompt := joinArgs(args)

	if chdir != "" {
		abs, err := filepath.Abs(chdir)
		if err != nil {
			panic(err)
		}
		chdir = abs
	}

	cmd := exec.Command(ag.bin)
	cmd.Env = os.Environ()
	if chdir != "" {
		cmd.Dir = chdir
	}

	ptmx, err := pty.Start(cmd)
	if err != nil {
		panic(err)
	}
	defer ptmx.Close()

	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		panic(err)
	}
	defer term.Restore(int(os.Stdin.Fd()), oldState)

	ch := make(chan os.Signal, 1)
	signal.Notify(ch, syscall.SIGWINCH)
	go func() {
		for range ch {
			ws, _ := pty.GetsizeFull(os.Stdin)
			if ws != nil {
				_ = pty.Setsize(ptmx, ws)
			}
		}
	}()
	ch <- syscall.SIGWINCH

	cmdDone := make(chan struct{})
	go func() {
		cmd.Wait()
		close(cmdDone)
	}()

	// delay stdin forwarding until after prompt is sent to avoid
	// spurious raw-mode bytes being interpreted as keypresses
	stdinEnabled := make(chan struct{})
	go func() {
		<-stdinEnabled
		io.Copy(ptmx, os.Stdin)
	}()

	if prompt == "" {
		close(stdinEnabled)
		io.Copy(os.Stdout, ptmx)
		return
	}

	injectedPrompt := prompt
	if autoExit && ag.formatterFn != nil {
		injectedPrompt = ag.formatterFn(prompt)
	}

	var outputBuf bytes.Buffer
	tee := io.TeeReader(ptmx, os.Stdout)

	readyMarker := regexp.MustCompile(ag.readyPattern)
	doneMarkerRe := regexp.MustCompile(regexp.QuoteMeta(doneMarker))
	idleRes := make([]*regexp.Regexp, len(ag.idlePatterns))
	for i, p := range ag.idlePatterns {
		idleRes[i] = regexp.MustCompile(p)
	}

	current := stateWaitingReady
	buf := make([]byte, 4096)
	var doneOnce sync.Once
	canCheckCompletion := false
	completionStarted := false

	exit := func() {
		doneOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "\n[pty-go] task complete, closing %s...\n", ag.name)
			time.Sleep(500 * time.Millisecond)
			ptmx.Write([]byte{0x03})
			time.Sleep(300 * time.Millisecond)
			ptmx.Write([]byte{0x03})
			time.Sleep(300 * time.Millisecond)
			cmd.Process.Signal(syscall.SIGTERM)
			time.Sleep(1 * time.Second)
			cmd.Process.Kill()
		})
	}

	transitionToWorking := func() {
		outputBuf.Reset()
		current = stateWorking
		time.AfterFunc(ag.gracePeriod, func() {
			canCheckCompletion = true
		})
	}

	fallback := time.AfterFunc(ag.fallbackTimeout, func() {
		if current == stateWaitingReady {
			current = stateSendingPrompt
			ag.sendPromptFn(ptmx, injectedPrompt)
			if autoExit {
				transitionToWorking()
			} else {
				close(stdinEnabled)
			}
		}
	})
	defer fallback.Stop()

	for {
		select {
		case <-cmdDone:
			return
		default:
		}

		n, err := tee.Read(buf)
		if err != nil {
			break
		}

		outputBuf.Write(buf[:n])

		switch current {
		case stateWaitingReady:
			stripped := stripANSI(outputBuf.String())
			if readyMarker.MatchString(stripped) {
				current = stateSendingPrompt
				fallback.Stop()
				time.AfterFunc(ag.readyWait, func() {
					ag.sendPromptFn(ptmx, injectedPrompt)
					if autoExit {
						transitionToWorking()
					} else {
						close(stdinEnabled)
					}
				})
			}

		case stateWorking:
			if !canCheckCompletion {
				continue
			}
			if !completionStarted {
				outputBuf.Reset()
				completionStarted = true
			}
			if outputBuf.Len() > 16384 {
				outputBuf.Next(outputBuf.Len() - 8192)
			}
			recent := stripANSI(outputBuf.String())

			if doneMarkerRe.MatchString(recent) {
				current = stateDone
				go exit()
				continue
			}

			for _, re := range idleRes {
				matches := re.FindAllStringIndex(recent, -1)
				if len(matches) >= 3 {
					current = stateDone
					go exit()
					break
				}
			}
		case stateDone:
		}
	}
}

func sendPromptTyped(ptmx *os.File, prompt string) {
	ptmx.Write([]byte{0x15}) // Ctrl+U
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte{0x17}) // Ctrl+W
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte(prompt))
	time.Sleep(100 * time.Millisecond)
	ptmx.Write([]byte{0x0d}) // Enter
}

// sendPromptForClaude sends prompt to Claude Code.
// No control characters — Claude Code's Ink TUI interprets them as interrupts.
// No newlines — Claude Code submits on Enter.
func sendPromptForClaude(ptmx *os.File, prompt string) {
	singleLine := strings.ReplaceAll(prompt, "\n", " ")
	singleLine = strings.ReplaceAll(singleLine, "\r", " ")

	ptmx.Write([]byte(singleLine))
	time.Sleep(300 * time.Millisecond)
	ptmx.Write([]byte{0x0d}) // Enter
}

var ansiRe = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]|\x1b\].*?\x07|\x1b\[.*?m`)

func stripANSI(s string) string {
	return ansiRe.ReplaceAllString(s, "")
}

func joinArgs(args []string) string {
	var buf bytes.Buffer
	for i, a := range args {
		if i > 0 {
			buf.WriteByte(' ')
		}
		buf.WriteString(a)
	}
	return buf.String()
}

func containsMarker(s string) bool {
	clean := stripANSI(s)
	return strings.Contains(clean, doneMarker)
}
