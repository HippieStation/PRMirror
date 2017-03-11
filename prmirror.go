package main

import (
	"context"
	"encoding/json"

	"time"

	"fmt"

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
	Database      *Database
}

func (p PRMirror) HandlePREvent(prEvent *github.PullRequestEvent) {
	downstreamPR, err := p.Database.GetDownstreamID(prEvent.PullRequest.GetNumber())
	if err != nil {
		panic(err)
	}

	prAction := prEvent.GetAction()

	log.Debugf("%s\n", prEvent.PullRequest.GetURL())

	if prAction == "opened" {
		// We already have a PR for this
		if downstreamPR != 0 {
			p.AddComment(downstreamPR, fmt.Sprintf("Upstream has reopened this PR"))
			p.RemoveLabel(downstreamPR, "Upstream Closed")
		} else {
			p.MirrorPR(prEvent.PullRequest)
		}
	} else if prAction == "closed" {
		if downstreamPR != 0 {
			if prEvent.PullRequest.GetMerged() == true {
				p.AddComment(downstreamPR, fmt.Sprintf("This PR has been merged upstream by %s", prEvent.PullRequest.MergedBy.GetName()))
				p.AddLabels(downstreamPR, []string{"Upstream Merged"})
			} else {
				p.AddLabels(downstreamPR, []string{"Upstream Closed"})
			}
		} else {
			panic("Upstream closed a PR we don't have, we are missing a pull request, something has gone wrong!")
		}
	}
}

func (p PRMirror) isRatelimit(err error) bool {
	if _, ok := err.(*github.RateLimitError); ok {
		// TODO: Maybe add some context here
		log.Error("The github.com rate limit has been hit")
		return true
	}
	return false
}

func (p PRMirror) GetOpenPRs() ([]*github.PullRequest, error) {
	var allPrs []*github.PullRequest

	opt := &github.PullRequestListOptions{
		ListOptions: github.ListOptions{PerPage: 100},
	}

	for {
		log.Debugf("Getting OpenPRs Page %d\n", opt.ListOptions.Page)

		prs, resp, err := p.GitHubClient.PullRequests.List(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, opt)
		if p.isRatelimit(err) {
			return nil, err
		}

		allPrs = append(allPrs, prs...)
		if resp.NextPage == 0 {
			break
		}
		opt.ListOptions.Page = resp.NextPage
	}

	return allPrs, nil
}

func (p PRMirror) InitialImport() {
	prs, err := p.GetOpenPRs()
	if p.isRatelimit(err) {
		return
	}

	for _, pr := range prs {
		prNum, err := p.Database.GetDownstreamID(pr.GetNumber())
		if err != nil {
			panic(err)
		}

		if prNum != 0 {
			log.Infof("DUP: [%d] - %s\n", pr.GetNumber(), pr.GetTitle())
		} else {
			log.Infof("NEW: [%d] - %s\n", pr.GetNumber(), pr.GetTitle())
			prID, err := p.MirrorPR(pr)
			if err != nil {
				panic(err)
			}
			p.Database.StoreMirror(prID, pr.GetNumber())
			p.AddLabels(prID, []string{"Upstream PR"})

			time.Sleep(2 * time.Second)
		}
	}
}

func (p PRMirror) Run() {
	events, _, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, nil)
	if p.isRatelimit(err) {
		return
	}

	for _, event := range events {
		eventType := event.GetType()

		if eventType == "PullRequestEvent" {
			prEvent := github.PullRequestEvent{}
			err = json.Unmarshal(event.GetRawPayload(), &prEvent)
			if err != nil {
				panic(err)
			}

			p.HandlePREvent(&prEvent)
		}
	}
}

func (p PRMirror) MirrorPR(pr *github.PullRequest) (int, error) {
	log.Infof("Mirroring PR [%d]: %s from ", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	base := "master"
	maintainerCanModify := false
	title := fmt.Sprintf("[MIRROR] %s", pr.GetTitle())
	body := fmt.Sprintf("Original PR: %s\n--------------------\n%s", pr.GetURL(), pr.GetBody())

	newPR := github.NewPullRequest{}
	newPR.Title = &title
	newPR.Body = &body
	newPR.Base = &base
	newPR.Head = pr.Head.Label
	newPR.MaintainerCanModify = &maintainerCanModify

	pr, _, err := p.GitHubClient.PullRequests.Create(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &newPR)
	if err != nil {
		return 0, err
	}

	return pr.GetNumber(), nil
}

func (p PRMirror) AddLabels(id int, tags []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamOwner, id, tags)
	if err != nil {
		log.Errorf("Error while adding a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

func (p PRMirror) RemoveLabel(id int, tags string) bool {
	_, err := p.GitHubClient.Issues.RemoveLabelForIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamOwner, id, tags)
	if err != nil {
		log.Errorf("Error while removing a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

func (p PRMirror) AddComment(id int, comment string) bool {
	issueComment := github.IssueComment{}
	issueComment.Body = &comment

	_, _, err := p.GitHubClient.Issues.CreateComment(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamOwner, id, &issueComment)
	if err != nil {
		log.Errorf("Error while adding a comment to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}
