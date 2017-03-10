package main

import (
	"context"
	"log"

	"fmt"

	"encoding/json"

	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

var Configuration = Config{}

func main() {
	Configuration = Configuration.Init()

	ctx := context.Background()
	ts := oauth2.StaticTokenSource(
		&oauth2.Token{AccessToken: Configuration.GitHubToken},
	)
	tc := oauth2.NewClient(ctx, ts)

	client := github.NewClient(tc)
	events, _, err := client.Activity.ListRepositoryEvents(ctx, Configuration.UpstreamOwner, Configuration.UpstreamRepo, nil)
	if _, ok := err.(*github.RateLimitError); ok {
		log.Println("hit rate limit")
	}

	for _, event := range events {

		if *event.Type == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}
			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}
			fmt.Printf("%s\n", *prEvent.PullRequest.URL)
		}
	}
}
