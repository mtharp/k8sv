package k8sv

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/ramr/go-reaper"
	"golang.org/x/sys/unix"
)

// Start reaping subprocesses and re-execute as a subprocess
func StartReaping() {
	if _, hasReaper := os.LookupEnv("REAPER"); hasReaper {
		return
	}
	go reaper.Reap()
	args := os.Args
	kidEnv := []string{fmt.Sprintf("REAPER=%d", os.Getpid())}
	var wstatus syscall.WaitStatus
	pattrs := &syscall.ProcAttr{
		Dir: "/",
		Env: append(os.Environ(), kidEnv...),
		Sys: &syscall.SysProcAttr{Setsid: true},
		Files: []uintptr{
			uintptr(syscall.Stdin),
			uintptr(syscall.Stdout),
			uintptr(syscall.Stderr),
		},
	}
	exe, err := os.Executable()
	if err != nil {
		log.Fatalln("error: finding myself:", exe)
	}
	pid, err := syscall.ForkExec(exe, args, pattrs)
	if err != nil {
		log.Fatalln("error:", err)
	}
	go PropagateSignals(pid)
	_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	for syscall.EINTR == err {
		_, err = syscall.Wait4(pid, &wstatus, 0, nil)
	}
	os.Exit(0)
}

// Launch a subprocess and propagate signals to it
func Launch(command []string) (*os.Process, error) {
	cmd := exec.Command(command[0], command[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		log.Fatalln("error:", err)
	}
	go PropagateSignals(cmd.Process.Pid)
	go func() {
		err := cmd.Wait()
		log.Fatalln("error: subprocess exited:", err)
	}()
	return cmd.Process, nil
}

// Listen for signals and propagate them to pid
func PropagateSignals(pid int) {
	sigs := make(chan os.Signal, 5)
	signal.Notify(sigs,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
		syscall.SIGINT,
		syscall.SIGUSR1,
		syscall.SIGUSR2,
	)
	for sig := range sigs {
		log.Printf("propagating %s to %d", sig, pid)
		unix.Kill(pid, sig.(syscall.Signal))
	}
}
