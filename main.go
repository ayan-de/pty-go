package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"sync"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

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
	var agentName string
	var allMode bool
	var paneMode bool
	var winMode bool
	var sessionName string
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
		case "-all":
			allMode = true
		case "-pane":
			paneMode = true
		case "-win":
			winMode = true
		case "-session":
			i++
			if i < len(os.Args) {
				sessionName = os.Args[i]
			}
		case "-codex":
			agentName = "codex"
		case "-opencode":
			agentName = "opencode"
		case "-claudecode", "-claude":
			agentName = "claudecode"
		case "-gemini":
			agentName = "gemini"
		default:
			args = append(args, os.Args[i])
		}
	}

	if allMode {
		if paneMode && winMode {
			os.Stderr.WriteString("error: -pane and -win are mutually exclusive\n")
			os.Exit(1)
		}
		if !paneMode && !winMode {
			os.Stderr.WriteString("error: specify -pane or -win with -all\n")
			os.Exit(1)
		}
		layout := "pane"
		if winMode {
			layout = "win"
		}
		prompt := JoinArgs(args)
		multiCfg := &multiAgentConfig{
			SessionName: sessionName,
			Layout:      layout,
			Agents:      []string{"opencode", "claudecode", "codex"},
			Prompt:      prompt,
			Chdir:       chdir,
			AutoExit:    autoExit,
		}
		if err := runMultiAgent(multiCfg); err != nil {
			os.Stderr.WriteString("error: " + err.Error() + "\n")
			os.Exit(1)
		}
		return
	}

	if agentName == "" {
		agentName = "opencode"
	}

	registry := NewRegistry()
	ag, ok := registry[agentName]
	if !ok {
		os.Stderr.WriteString("unknown agent: " + agentName + "\n")
		os.Exit(1)
	}

	prompt := JoinArgs(args)

	if chdir != "" {
		abs, err := filepath.Abs(chdir)
		if err != nil {
			panic(err)
		}
		chdir = abs
	}

	cmd := exec.Command(ag.Bin, ag.Args...)
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
	if autoExit && ag.FormatPrompt != nil {
		injectedPrompt = ag.FormatPrompt(prompt)
	}

	var outputBuf bytes.Buffer
	tee := io.TeeReader(ptmx, os.Stdout)

	readyMarker := regexp.MustCompile(ag.ReadyPattern)
	doneMarkerRe := regexp.MustCompile(regexp.QuoteMeta(doneMarker))
	idleRes := make([]*regexp.Regexp, len(ag.IdlePatterns))
	for i, p := range ag.IdlePatterns {
		idleRes[i] = regexp.MustCompile(p)
	}

	current := stateWaitingReady
	buf := make([]byte, 4096)
	var doneOnce sync.Once
	canCheckCompletion := false
	completionStarted := false

	exit := func() {
		doneOnce.Do(func() {
			os.Stderr.WriteString("\n[pty-go] task complete, closing " + ag.Name + "...\n")
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
		time.AfterFunc(ag.GracePeriod, func() {
			canCheckCompletion = true
		})
	}

	fallback := time.AfterFunc(ag.FallbackTimeout, func() {
		if current == stateWaitingReady {
			current = stateSendingPrompt
			ag.SendPrompt(ptmx, injectedPrompt)
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
			stripped := StripANSI(outputBuf.String())
			if readyMarker.MatchString(stripped) {
				current = stateSendingPrompt
				fallback.Stop()
				time.AfterFunc(ag.ReadyWait, func() {
					ag.SendPrompt(ptmx, injectedPrompt)
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
			recent := StripANSI(outputBuf.String())

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
