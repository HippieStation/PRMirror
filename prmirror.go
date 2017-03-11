package main

import (
	"context"
	"encoding/json"

	"github.com/google/go-github/github"
)

type MirroredPR struct {
	DownstreamID int
	UpstreamID   int
}

type PRMirror struct {
	GitHubClient  *github.Client
	Context       *context.Context
	Configuration *Config
}

func (p PRMirror) Run() {
	events, _, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, nil)
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
	}

	for _, event := range events {

		if *event.Type == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}

			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}

			prAction := prEvent.GetAction()

			log.Debugf("%s\n", prEvent.PullRequest.GetURL())

			if prAction == "opened" {
				//TODO: Check if we already have an open PR for this and add a comment saying upstream reopened it and remove the upsteam closed tag
				p.MirrorPR(&prEvent)
			} else if prAction == "closed" {

				//AddLabel("Upstream Closed")
			}
		}
	}
}

func (p PRMirror) MirrorPR(PREvent *github.PullRequestEvent) {
	log.Infof("Mirroring PR [%d]: %s from ", PREvent.PullRequest.GetNumber(), PREvent.PullRequest.GetTitle(), PREvent.PullRequest.User.GetLogin())
}

func (p PRMirror) AddLabels(id int, tags []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, id, tags)
	if err != nil {
		log.Errorf("Error while adding a label to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}
