package main

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"regexp"
	"syscall"
	"time"

	"github.com/creack/pty"
	"golang.org/x/term"
)

func main() {
	var chdir string
	var args []string
	for i := 1; i < len(os.Args); i++ {
		if os.Args[i] == "-chdir" && i+1 < len(os.Args) {
			chdir = os.Args[i+1]
			i++
		} else {
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

	go func() {
		io.Copy(ptmx, os.Stdin)
	}()

	if prompt == "" {
		io.Copy(os.Stdout, ptmx)
		return
	}

	var outputBuf bytes.Buffer
	tee := io.TeeReader(ptmx, os.Stdout)

	readyMarker := regexp.MustCompile(`Ask\s+anything`)
	ready := false
	buf := make([]byte, 4096)

	// fallback: send prompt after 5s even if marker not seen
	fallback := time.AfterFunc(5*time.Second, func() {
		if !ready {
			ready = true
			sendPrompt(ptmx, prompt)
		}
	})
	defer fallback.Stop()

	for {
		n, err := tee.Read(buf)
		if err != nil {
			break
		}
		if !ready {
			outputBuf.Write(buf[:n])
			stripped := stripANSI(outputBuf.String())
			if readyMarker.MatchString(stripped) {
				ready = true
				fallback.Stop()
				time.AfterFunc(500*time.Millisecond, func() {
					sendPrompt(ptmx, prompt)
				})
			}
		}
	}
}

func sendPrompt(ptmx *os.File, prompt string) {
	ptmx.Write([]byte{0x15}) // Ctrl+U: clear input line
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte{0x17}) // Ctrl+W: clear word (extra safety)
	time.Sleep(50 * time.Millisecond)
	ptmx.Write([]byte(prompt))
	time.Sleep(100 * time.Millisecond)
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
