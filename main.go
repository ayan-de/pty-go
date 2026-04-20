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

func main() {
	var chdir string
	var autoExit bool
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
		default:
			args = append(args, os.Args[i])
		}
	}

	prompt := joinArgs(args)

	if chdir != "" {
		abs, err := filepath.Abs(chdir)
		if err != nil {
			panic(err)
		}
		chdir = abs
	}

	cmd := exec.Command("opencode")
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

	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	if prompt == "" {
		io.Copy(os.Stdout, ptmx)
		return
	}

	injectedPrompt := prompt
	if autoExit {
		injectedPrompt = fmt.Sprintf(
			"%s\n\nIMPORTANT: After you have fully completed all the above tasks, you MUST print exactly this line on its own: %s. Do not skip this.",
			prompt, doneMarker,
		)
	}

	var outputBuf bytes.Buffer
	tee := io.TeeReader(ptmx, os.Stdout)

	readyMarker := regexp.MustCompile(`Ask\s+anything`)
	doneMarkerRe := regexp.MustCompile(regexp.QuoteMeta(doneMarker))
	askAfterWork := regexp.MustCompile(`Ask\s+anything`)

	current := stateWaitingReady
	buf := make([]byte, 4096)
	var doneOnce sync.Once
	canCheckCompletion := false

	exit := func() {
		doneOnce.Do(func() {
			fmt.Fprintf(os.Stderr, "\n[pty-go] task complete, closing...\n")
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
		time.AfterFunc(8*time.Second, func() {
			canCheckCompletion = true
		})
	}

	fallback := time.AfterFunc(5*time.Second, func() {
		if current == stateWaitingReady {
			current = stateSendingPrompt
			sendPrompt(ptmx, injectedPrompt)
			if autoExit {
				transitionToWorking()
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
				time.AfterFunc(500*time.Millisecond, func() {
					sendPrompt(ptmx, injectedPrompt)
					if autoExit {
						transitionToWorking()
					}
				})
			}

		case stateWorking:
			if !canCheckCompletion {
				continue
			}
			if outputBuf.Len() > 16384 {
				outputBuf.Next(outputBuf.Len() - 8192)
			}
			recent := stripANSI(outputBuf.String())

			// Strategy 1: marker in output
			if doneMarkerRe.MatchString(recent) {
				current = stateDone
				go exit()
				continue
			}

			// Strategy 2: "Ask anything" reappeared after work started
			// Must see it multiple times to confirm it's truly idle
			if askAfterWork.MatchString(recent) {
				// count how many times "Ask anything" appears
				matches := askAfterWork.FindAllStringIndex(recent, -1)
				if len(matches) >= 3 {
					current = stateDone
					go exit()
				}
			}
		case stateDone:
		}
	}
}

func sendPrompt(ptmx *os.File, prompt string) {
	ptmx.Write([]byte{0x15})
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte{0x17})
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte(prompt))
	time.Sleep(100 * time.Millisecond)
	ptmx.Write([]byte{0x0d})
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

// check if string contains the marker ignoring ANSI and whitespace
func containsMarker(s string) bool {
	clean := stripANSI(s)
	return strings.Contains(clean, doneMarker)
}
