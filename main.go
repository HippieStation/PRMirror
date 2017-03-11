package main

import (
	"context"
	"os"

	"flag"

	"github.com/google/go-github/github"
	"github.com/op/go-logging"
	"golang.org/x/oauth2"
)

var PRMirrorer = PRMirror{}
var InitialImport = false
var DumpDB = false

func init() {
	flag.BoolVar(&InitialImport, "InitialImport", false, "Import all current PRs from Upstream")
	flag.BoolVar(&DumpDB, "DumpDB", false, "Dump the database to stdout")

	flag.Parse()
}

func main() {
	backend := logging.NewLogBackend(os.Stderr, "", 0)
	backendFormatter := logging.NewBackendFormatter(backend, format)
	logging.SetBackend(backendFormatter)

	Configuration := Config{}.Init()

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
	}

	if InitialImport {
		PRMirrorer.CreateLabel("Upstream PR Open", "28a745")
		PRMirrorer.CreateLabel("Upstream PR Closed", "cb2431")
		PRMirrorer.CreateLabel("Upstream PR Merged", "6f42c1")

		PRMirrorer.InitialImport()
	} else if DumpDB {
		PRMirrorer.Database.DumpDB()
	} else {
		PRMirrorer.Run()
	}

	defer Database.Close()
}
