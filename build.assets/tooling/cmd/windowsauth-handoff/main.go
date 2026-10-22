/*
 * Teleport
 * Copyright (C) 2026  Gravitational, Inc.
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU Affero General Public License as published by
 * the Free Software Foundation, either version 3 of the License, or
 * (at your option) any later version.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU Affero General Public License for more details.
 *
 * You should have received a copy of the GNU Affero General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 */

package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/alecthomas/kingpin/v2"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/sts"
	"github.com/gravitational/trace"
	"go.yaml.in/yaml/v3"

	awsconfig "github.com/gravitational/teleport/lib/cloud/aws/config"
	"github.com/gravitational/teleport/lib/utils/aws/stsutils"
)

type environmentConfig struct {
	Name           string `yaml:"-"`
	Account        string `yaml:"-"`
	UnsignedBucket string `yaml:"WINDOWSAUTH_UNSIGNED_BUCKET"`
	SignedBucket   string `yaml:"WINDOWSAUTH_SIGNED_BUCKET"`
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	app := kingpin.New("windowsauth-handoff", "Download Windows auth CABs and upload Microsoft signing submissions.")
	environment := app.Flag("environment", "AWS environment.").Required().Enum("prod", "stage")
	profile := app.Flag("profile", "AWS shared configuration profile; defaults to the AWS credential chain.").String()
	environmentsDir := app.Flag("environments-dir", "Path to the environment configuration directory.").Default("e/.environments").String()
	download := app.Command("download", "Download exactly one CAB set into an empty directory.")
	version := download.Flag("version", "Windows auth version, with or without v/V.").Required().String()
	out := download.Flag("out", "Empty output directory, created if missing.").Required().String()
	upload := app.Command("upload", "Upload the Microsoft ZIP and companion submission.json to the signed bucket.")
	handoffDir := upload.Flag("dir", "Directory produced by download.").Required().String()
	zipPath := upload.Flag("zip", "Microsoft-returned Signed_<id>.zip.").Required().String()
	note := upload.Flag("note", "Optional operator note recorded in submission.json.").String()

	command, err := app.Parse(args)
	if err != nil {
		return trace.Wrap(err)
	}

	// get bucket names and expected AWS accounts from `e/.environments/<env>/build/windows-signing.yaml`
	env, err := loadEnvironment(*environment, *environmentsDir)
	if err != nil {
		return trace.Wrap(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	ctx, cancel := context.WithTimeout(ctx, 15*time.Minute)
	defer cancel()

	options := []func(*config.LoadOptions) error{config.WithRegion("us-west-2")}
	if *profile != "" {
		options = append(options, config.WithSharedConfigProfile(*profile))
	}

	awsConfig, err := awsconfig.LoadDefaultConfig(ctx, options...)
	if err != nil {
		return trace.Wrap(err, "load AWS configuration")
	}

	caller, err := authenticate(ctx, stsutils.NewFromConfig(awsConfig), env.Account, *profile)
	if err != nil {
		return trace.Wrap(err)
	}

	fmt.Fprintf(os.Stderr, "AWS caller: %s\n", caller)

	handoff := handoffTool{store: s3.NewFromConfig(awsConfig), env: env, caller: caller, now: time.Now}

	switch command {
	case download.FullCommand():
		err = handoff.download(ctx, *version, *out)
	case upload.FullCommand():
		err = handoff.upload(ctx, *handoffDir, *zipPath, *note)
	}
	return trace.Wrap(err)
}

type identityClient interface {
	GetCallerIdentity(context.Context, *sts.GetCallerIdentityInput, ...func(*sts.Options)) (*sts.GetCallerIdentityOutput, error)
}

func authenticate(ctx context.Context, client identityClient, account, profile string) (string, error) {
	identity, err := client.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		login := "aws sso login"
		if profile != "" {
			login += fmt.Sprintf(" --profile %q", profile)
		}
		return "", trace.Wrap(err, "AWS identity check failed; if using SSO, refresh the session with %s", login)
	}
	if identity.Account == nil || *identity.Account != account {
		return "", trace.AccessDenied("AWS credentials must belong to account %s", account)
	}
	if identity.Arn == nil || *identity.Arn == "" {
		return "", trace.BadParameter("AWS returned an empty caller ARN")
	}
	return *identity.Arn, nil
}

func loadEnvironment(name, directory string) (environmentConfig, error) {
	accounts := map[string]string{"prod": "146628656107", "stage": "603330915152"}
	account, ok := accounts[name]
	if !ok {
		return environmentConfig{}, trace.BadParameter("unknown environment %q", name)
	}
	if directory == "" {
		directory = "e/.environments"
	}
	filename := filepath.Join(directory, name, "build", "windows-signing.yaml")
	data, err := os.ReadFile(filename)
	if err != nil {
		return environmentConfig{}, trace.Wrap(err, "read environment configuration; run from the repository root or set --environments-dir (use .environments from core/e)")
	}
	var env environmentConfig
	if err := yaml.Unmarshal(data, &env); err != nil {
		return environmentConfig{}, trace.Wrap(err)
	}
	if env.UnsignedBucket == "" || env.SignedBucket == "" {
		return environmentConfig{}, trace.BadParameter("windows-signing.yaml must define unsigned and signed buckets")
	}
	env.Name, env.Account = name, account
	return env, nil
}
