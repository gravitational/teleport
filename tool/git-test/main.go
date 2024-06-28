package main

import (
	"io"
	"os"
	"os/exec"
)

func main() {
	cmd := exec.Command("git", "remote-http", "origin", "https://teleport.dev.aws.stevexin.me/v1/repos/steve-codecommit")
	cmd.Stdin = os.Stdin
	cmd.Stdout = io.MultiWriter(os.Stdout, os.Stderr)
	cmd.Stderr = os.Stderr
	cmd.Env = append(os.Environ(),
		"GIT_SSL_CERT=/Users/stevehuang/.tsh/keys/teleport.dev.aws.stevexin.me/STeve-app/teleport.dev.aws.stevexin.me/aws-dev-account-x509.pem",
		"GIT_SSL_KEY=/Users/stevehuang/.tsh/keys/teleport.dev.aws.stevexin.me/STeve",
		"GIT_CONFIG_COUNT=1",
		"GIT_CONFIG_KEY_0=http.extraHeader",
		"GIT_CONFIG_VALUE_0=X-Teleport-Original-Git-Url: https://git-codecommit.ca-central-1.amazonaws.com/v1/repos/steve-codecommit",
	)
	cmd.Run()
	os.Exit(cmd.ProcessState.ExitCode())
}
