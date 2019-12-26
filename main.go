package main

import (
	"context"
	"os"

	"flag"

	"fmt"

	"github.com/google/go-github/github"
	logging "github.com/op/go-logging"
	"golang.org/x/oauth2"
)

// PRMirrorer represents the PRMirror engine
var PRMirrorer = PRMirror{}

// CreateLabels do we create labels for PRs (read from config)
var CreateLabels = false

// DumpDB do we dump the contents of the DB to stdout (read from config)
var DumpDB = false

func init() {
	flag.BoolVar(&CreateLabels, "CreateLabels", false, "Create all of the labels")
	flag.BoolVar(&DumpDB, "DumpDB", false, "Dump the database to stdout")

	flag.Parse()
}

func main() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	Configuration := Config{}.Init()

	if _, err := os.Stat(fmt.Sprintf("%s%s", Configuration.RepoPath, Configuration.ToolPath)); os.IsNotExist(err) {
		log.Errorf("Could not find the shell script located at %s%s\n", Configuration.RepoPath, Configuration.ToolPath)
		return
	}

	Database := NewDatabase()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: Configuration.GitHubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	client.UserAgent = "HippieStation/PRMirror"

	PRMirrorer = PRMirror{
		GitHubClient:  client,
		Context:       &ctx,
		Configuration: &Configuration,
		Database:      Database,
		GitLock:       &SpinLock{},
	}

	if CreateLabels {
		PRMirrorer.CreateLabel("Upstream PR Open", "28a745")
		PRMirrorer.CreateLabel("Upstream PR Closed", "cb2431")
		PRMirrorer.CreateLabel("Upstream PR Merged", "6f42c1")
	} else if DumpDB {
		PRMirrorer.Database.DumpDB()
	}

	if Configuration.UseWebhook {
		log.Info("Using the webhook listener\n")
		PRMirrorer.RunWebhookListener()
	} else {
		log.Info("Using the event scraper\n")
		PRMirrorer.RunEventScraper()
	}

	defer Database.Close()
}
