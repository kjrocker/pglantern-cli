package output

import (
	"os"
	"os/exec"
	"strings"
)

// pager is the running pager process, nil when none was started.
var pager *pagerProc

type pagerProc struct {
	cmd  *exec.Cmd
	w    *os.File
	orig *os.File
}

// StartPager pipes os.Stdout through $LANTERN_PAGER, $PAGER, or `less -FRX`
// for interactive runs. Reassigning the os.Stdout variable covers every render
// path, since they all resolve os.Stdout at call time. No-op when stdout isn't
// a TTY, the resolved pager binary is missing, or a pager is already running.
// `less -FRX` exits immediately when output fits one screen, so short tables
// feel unpaged.
func StartPager() {
	if pager != nil || !Interactive {
		return
	}
	argv := []string{"less", "-FRX"}
	for _, env := range []string{"LANTERN_PAGER", "PAGER"} {
		if v := strings.TrimSpace(os.Getenv(env)); v != "" {
			argv = strings.Fields(v)
			break
		}
	}
	path, err := exec.LookPath(argv[0])
	if err != nil {
		return
	}
	r, w, err := os.Pipe()
	if err != nil {
		return
	}
	cmd := exec.Command(path, argv[1:]...)
	cmd.Stdin = r
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		r.Close()
		w.Close()
		return
	}
	r.Close()
	pager = &pagerProc{cmd: cmd, w: w, orig: os.Stdout}
	os.Stdout = w
}

// StopPager closes the pager's input, restores os.Stdout, and waits for the
// pager to exit (it holds the terminal until the user quits). Idempotent, so
// both the post-run hook and main's error path may call it.
func StopPager() {
	if pager == nil {
		return
	}
	pager.w.Close()
	os.Stdout = pager.orig
	pager.cmd.Wait()
	pager = nil
}
