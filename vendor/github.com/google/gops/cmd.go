package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"

	"github.com/google/gops/internal"
	"github.com/google/gops/signal"
	ps "github.com/keybase/go-ps"
)

var cmds = map[string](func(pid int) error){
	"stack":      stackTrace,
	"gc":         gc,
	"memstats":   memStats,
	"version":    version,
	"pprof-heap": pprofHeap,
	"pprof-cpu":  pprofCPU,
	"stats":      stats,
}

func stackTrace(pid int) error {
	return cmdWithPrint(pid, signal.StackTrace)
}

func gc(pid int) error {
	_, err := cmd(pid, signal.GC)
	return err
}

func memStats(pid int) error {
	return cmdWithPrint(pid, signal.MemStats)
}

func version(pid int) error {
	return cmdWithPrint(pid, signal.Version)
}

func pprofHeap(pid int) error {
	return pprof(pid, signal.HeapProfile)
}

func pprofCPU(pid int) error {
	fmt.Println("Profiling CPU now, will take 30 secs...")
	return pprof(pid, signal.CPUProfile)
}

func pprof(pid int, p byte) error {
	out, err := cmd(pid, p)
	if err != nil {
		return err
	}
	if len(out) == 0 {
		return errors.New("failed to read the profile")
	}
	tmpfile, err := ioutil.TempFile("", "profile")
	if err != nil {
		return err
	}
	defer os.Remove(tmpfile.Name())
	if err := ioutil.WriteFile(tmpfile.Name(), out, 0); err != nil {
		return err
	}
	process, err := ps.FindProcess(pid)
	if err != nil {
		// TODO(jbd): add context to the error
		return err
	}
	binary, err := process.Path()
	if err != nil {
		return fmt.Errorf("cannot the binary for the PID: %v", err)
	}
	cmd := exec.Command("go", "tool", "pprof", binary, tmpfile.Name())
	cmd.Env = os.Environ()
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func stats(pid int) error {
	return cmdWithPrint(pid, signal.Stats)
}

func cmdWithPrint(pid int, c byte) error {
	out, err := cmd(pid, c)
	if err != nil {
		return err
	}
	fmt.Printf("%s", out)
	return nil
}

func cmd(pid int, c byte) ([]byte, error) {
	port, err := internal.GetPort(pid)
	if err != nil {
		return nil, err
	}
	conn, err := net.Dial("tcp", "127.0.0.1:"+port)
	if err != nil {
		return nil, err
	}
	if _, err := conn.Write([]byte{c}); err != nil {
		return nil, err
	}
	all, err := ioutil.ReadAll(conn)
	if err != nil {
		return nil, err
	}
	return all, nil
}
