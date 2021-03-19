package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"eaglesong.dev/k8sv"
	"github.com/fsnotify/fsnotify"
	"golang.org/x/sys/unix"
)

func main() {
	var j int
	for i, arg := range os.Args[1:] {
		if arg == "--" {
			j = i + 1
		}
	}
	if j == 0 {
		log.Fatalln("usage: reloader files+ -- subcommand")
	}
	k8sv.StartReaping()
	files := os.Args[1:j]
	command := os.Args[j+1:]
	proc, err := k8sv.Launch(command)
	if err != nil {
		log.Fatalln("error:", err)
	}
	if err := watch(files, proc); err != nil {
		log.Fatalln("error:", err)
	}
}

func watch(dirs []string, proc *os.Process) error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	for _, arg := range dirs {
		if err := watcher.Add(arg); err != nil {
			return fmt.Errorf("%s: %w", arg, err)
		}
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return errors.New("watcher ended")
			}
			if event.Op&fsnotify.Create != 0 && filepath.Base(event.Name) == "..data" {
				log.Println(event.Name, "changed, sending SIGHUP to", proc.Pid)
				proc.Signal(unix.SIGHUP)
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				return errors.New("watcher ended")
			}
			return err
		}
	}
}
