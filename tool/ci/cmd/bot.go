package main

import (
	"context"
	"flag"
	"log"
	"os"
	"strings"

	"github.com/gravitational/teleport/tool/ci"
	"github.com/gravitational/teleport/tool/ci/pkg/bot"
	bots "github.com/gravitational/teleport/tool/ci/pkg/bot"
	"github.com/gravitational/teleport/tool/ci/pkg/environment"
	"github.com/gravitational/trace"

	"github.com/google/go-github/v37/github"
	"golang.org/x/oauth2"
)

func main() {
	var token = flag.String("token", "", "token is the Github authentication token.")
	var reviewers = flag.String("reviewers", "", "reviewers is a string representing a json object that maps authors to required reviewers for that author.")
	flag.Parse()

	subcommand := os.Args[len(os.Args)-1]
	ctx := context.Background()

	switch subcommand {
	case ci.Assign:
		log.Println("Assigning reviewers")
		bot, err := constructBot(ctx, *token, *reviewers)
		if err != nil {
			log.Fatal(err)
		}
		err = bot.Assign(ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("Assign completed")

	case ci.Check:
		log.Println("Checking reviewers")
		bot, err := constructBot(ctx, *token, *reviewers)
		if err != nil {
			log.Fatal(err)
		}
		err = bot.Check(ctx)
		if err != nil {
			log.Fatal(err)
		}
		log.Print("Check completed")
	case ci.Dismiss:
		log.Println("Dismissing stale runs")
		err := dismissRuns(ctx, *token)
		if err != nil {
			log.Fatal(err)
		}
		log.Println("Stale workflow run removal completed")
	default:
		log.Fatalf("Unknown subcommand: %v.\nThe following subcommands are supported:\n"+
			"\tassign-reviewers \n\t assigns reviewers to a pull request.\n"+
			"\tcheck-reviewers \n\t checks pull request for required reviewers.\n"+
			"\tdismiss-runs \n\t dismisses stale workflow runs for external contributors.\n", subcommand)
	}

}

func constructBot(ctx context.Context, token, reviewers string) (*bots.Bot, error) {
	path := os.Getenv(ci.GithubEventPath)
	env, err := environment.New(environment.Config{Client: makeGithubClient(ctx, token),
		Reviewers: reviewers,
		EventPath: path,
		Token:     token,
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

func dismissRuns(ctx context.Context, token string) error {
	repository := os.Getenv(ci.GithubRepository)
	if repository == "" {
		return trace.BadParameter("environment variable GITHUB_REPOSITORY is not set")
	}
	metadata := strings.Split(repository, "/")
	if len(metadata) != 2 {
		return trace.BadParameter("environment variable GITHUB_REPOSITORY is not in the correct format,\n the valid format is '<repo owner>/<repo name>'")
	}
	clt := makeGithubClient(ctx, token)
	githubClient := bot.GithubClient{Client: clt}
	err := githubClient.DimissStaleWorkflowRunsForExternalContributors(ctx, token, metadata[0], metadata[1])
	if err != nil {
		return trace.Wrap(err)
	}
	return nil
}

func makeGithubClient(ctx context.Context, token string) *github.Client {
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: token},
	)
	tc := oauth2.NewClient(ctx, ts)
	return github.NewClient(tc)
}
