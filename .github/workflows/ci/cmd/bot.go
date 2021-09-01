package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/gravitational/teleport/.github/workflows/ci"
	bots "github.com/gravitational/teleport/.github/workflows/ci/pkg/bot"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/cron"
	"github.com/gravitational/teleport/.github/workflows/ci/pkg/environment"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
	"golang.org/x/oauth2"
)

func main() {
	var token = flag.String("token", "", "token is the Github authentication token.")
	var reviewers = flag.String("reviewers", "", "reviewers is a string representing a json object that maps authors to required reviewers for that author.")
	var defaultReviewers = flag.String("default-reviewers", "", "default-reviewers represents reviewers for external contributors or any author that does not have a key-value pair in '--reviewers'.")
	flag.Parse()
	subcommand := os.Args[len(os.Args)-1]
	if *token == "" {
		log.Fatal("Missing authentication token.")
	}
	switch subcommand {
	case ci.ASSIGN:
		log.Println("Assigning reviewers.")
		bot, err := verifyAndConstruct(*token, *reviewers, *defaultReviewers)
		if err != nil {
			log.Fatal(err)
		}
		err = bot.Assign()
		if err != nil {
			log.Fatal(err)
		}
		log.Print("Assign completed.")

	case ci.CHECK:
		log.Println("Checking reviewers.")
		bot, err := verifyAndConstruct(*token, *reviewers, *defaultReviewers)
		if err != nil {
			log.Fatal(err)
		}
		err = bot.Check()
		if err != nil {
			log.Fatal(err)
		}
		log.Print("Check completed.")
	case ci.CRON:
		repository := os.Getenv("GITHUB_REPOSITORY")
		if repository == "" {
			log.Fatal("Environment variable GITHUB_REPOSITORY is not set.")
		}
		metadata := strings.Split(repository, "/")
		if len(metadata) != 2 {
			log.Fatal("Environment variable GITHUB_REPOSITORY is not in the correct format,\n valid format is '<repo owner>/<repo name>'.")
		}
		err := cron.DimissStaleWorkflowRunsForExternalContributors(*token, metadata[0], metadata[1], makeGithubClient(*token))
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Stale workflow run removal completed. ")
	default:
		log.Fatalf("Unknown subcommand: %v", subcommand)
	}

}

func verifyAndConstruct(token, reviewers, defaultReviewers string) (*bots.Bot, error) {
	if token == "" {
		return nil, trace.BadParameter("Missing authentication token.")
	}
	if reviewers == "" {
		return nil, trace.BadParameter("Missing assignments flag.")
	}
	if defaultReviewers == "" {
		return nil, trace.BadParameter("Missing default-reviewers flag.")
	}

	path := os.Getenv(ci.GITHUBEVENTPATH)
	if path == "" {
		return nil, trace.BadParameter("Environment variable GITHUB_EVENT_PATH is not set.")
	}
	env, err := environment.New(environment.Config{Client: makeGithubClient(token),
		Reviewers:        reviewers,
		DefaultReviewers: defaultReviewers,
		EventPath:        path,
		Token:            token,
	})
	if err != nil {
		return nil, trace.Wrap(err)
	}

	bot, err := bots.New(bots.Config{Environment: env})
	if err != nil {
		return nil, trace.Wrap(err)
	}
	return bot, nil
}

func makeGithubClient(token string) *github.Client {
	// Creating and authenticating the Github client
	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
