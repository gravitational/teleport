//+build tools

// Run this script like so:
//
//	go run makefile.go <cmd> <args>...
//

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/shabbyrobe/gocovmerge"
	"golang.org/x/tools/cover"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	var commandNames = []string{
		"cover",
	}
	if len(os.Args) < 2 {
		return fmt.Errorf("command missing: expected %s", commandNames)
	}

	command, args := os.Args[1], os.Args[2:]
	switch command {
	case "cover":
		return runCover(args)
	case "builddocker":
		return runBuildDocker()
	case "buildrelease":
		return runBuildRelease()
	case "release":
		return runRelease(args)
	default:
		return fmt.Errorf("unknown command %v: expected %s", command, commandNames)
	}

	return nil
}

func runBuildDocker() error {
	cmd := command(
		"go", "build", "-a",
		"-installsuffix", "cgo",
		"-o", "./build/main",
		"./cmd/gofakes3",
	)
	cmd.Env = append(cmd.Env, "GO111MODULE=on", "CGO_ENABLED=0", "GOOS=linux")
	if err := cmd.Run(); err != nil {
		return err
	}

	cmd = command("docker", "build", "-t", "johannesboyne/gofakes3", ".")
	if err := cmd.Run(); err != nil {
		return err
	}

	return nil
}

func runRelease(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("missing <version> argument")
	}

	version := args[0]
	fmt.Printf("[+] releasing %s\n", version)
	fmt.Println("[+] re-generating")

	fmt.Println("[+] building")
	if err := runBuildRelease(); err != nil {
		return err
	}

	fmt.Println("[+] comitting")
	if err := command("git", "tag", version).Run(); err != nil {
		return err
	}

	fmt.Println("[+] complete")
	return nil
}

func runBuildRelease() error {
	var builds = map[string][]string{
		"linux_amd64":       {"GOOS=linux", "GOARCH=amd64"},
		"linux_arm7":        {"GOOS=linux", "GOARCH=arm", "GOARM=7"},
		"darwin_amd64":      {"GOOS=darwin", "GOARCH=amd64"},
		"windows_amd64.exe": {"GOOS=windows", "GOARCH=amd64"},
	}

	for suffix, build := range builds {
		cmd := command(
			"go", "build",
			"-o", fmt.Sprintf("./build/gofakes3_%s", suffix),
			"./cmd/gofakes3",
		)
		cmd.Env = append(cmd.Env, "GO111MODULE=on")
		cmd.Env = append(cmd.Env, build...)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	return nil
}

// runCover collects true code coverage for all packages in gofakes3.
// It does so by running 'go test' for each child package (enumerated by
// 'go list ./...') with the '-coverpkg' flag, populated with the same
// 'go list'.
func runCover(args []string) error {
	pkgs := goList()

	var files []string

	for _, pkg := range pkgs {
		covFile, err := ioutil.TempFile("", "")
		if err != nil {
			return err
		}
		covFile.Close()
		defer os.Remove(covFile.Name())

		files = append(files, covFile.Name())
		cmd := exec.Command("go", "test",
			"-covermode=atomic",
			fmt.Sprintf("-coverprofile=%s", covFile.Name()),
			fmt.Sprintf("-coverpkg=%s", strings.Join(pkgs, ",")),
			pkg,
		)
		if err := cmd.Run(); err != nil {
			return err
		}
	}

	var merged []*cover.Profile
	for _, file := range files {
		profiles, err := cover.ParseProfiles(file)
		if err != nil {
			return fmt.Errorf("failed to parse profiles: %v", err)
		}
		for _, p := range profiles {
			merged = gocovmerge.AddProfile(merged, p)
		}
	}

	var out io.WriteCloser = os.Stdout
	if len(args) > 0 {
		var err error
		out, err = os.Create(args[0])
		if err != nil {
			return err
		}
	}
	defer out.Close()

	return gocovmerge.DumpProfiles(merged, out)
}

func command(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = append([]string{}, os.Environ()...)
	return cmd
}

func goList() (pkgs []string) {
	cmd := exec.Command("go", "list", "./...")

	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	if err := cmd.Run(); err != nil {
		panic(err)
	}

	scanner := bufio.NewScanner(&stdout)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if len(line) == 0 {
			continue
		}
		pkgs = append(pkgs, line)
	}
	return pkgs
}
