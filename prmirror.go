package main

import (
	"context"
	"encoding/json"

	"time"

	"fmt"

	"strconv"

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

	prAction := prEvent.GetAction()

	log.Debugf("%s\n", prEvent.PullRequest.GetURL())

	if prAction == "closed" {
		if prEvent.PullRequest.GetMerged() == true {
			prID, err := p.MirrorPR(prEvent.PullRequest)
			if err != nil {
				log.Errorf("Error while creating a new PR: %s\n", err.Error())
			} else {
				p.AddLabels(prID, []string{"Upstream PR Merged"})
				p.Database.StoreMirror(prID, prEvent.PullRequest.GetNumber())
			}
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

func (p PRMirror) GetRepoEvents() ([]*github.Event, int64, error) {
	var allEvents []*github.Event
	var pollInterval = int64(0)

	opt := &github.ListOptions{
		PerPage: 100,
	}

	for {
		log.Debugf("Getting RepoEvents Page %d\n", opt.Page)

		events, resp, err := p.GitHubClient.Activity.ListRepositoryEvents(*p.Context, p.Configuration.UpstreamOwner, p.Configuration.UpstreamRepo, opt)
		if err != nil {
			log.Errorf("Error while listing repository events. %s", err.Error())

			pollInterval, err = strconv.ParseInt(resp.Response.Header.Get("X-Poll-Interval"), 10, 64)
			if err != nil {
				panic(err)
			}

			return nil, pollInterval, err
		}

		allEvents = append(allEvents, events...)
		if resp.NextPage == 0 {
			pollInterval, err = strconv.ParseInt(resp.Response.Header.Get("X-Poll-Interval"), 10, 64)
			if err != nil {
				panic(err)
			}
			break
		}
		opt.Page = resp.NextPage
	}

	return allEvents, pollInterval, nil
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
				log.Errorf("Error while creating a new PR: %s\n", err.Error())
			} else {
				p.Database.StoreMirror(prID, pr.GetNumber())
				p.AddLabels(prID, []string{"Upstream PR Open"})
			}
			time.Sleep(2 * time.Second)
		}
	}
}

func (p PRMirror) Run() {
	for {
		events, pollInterval, err := p.GetRepoEvents()
		if err == nil {
			for _, event := range events {
				seenEvent, _ := p.Database.SeenEvent(event.GetID())

				if !seenEvent {
					eventType := event.GetType()

					if eventType == "PullRequestEvent" {
						prEvent := github.PullRequestEvent{}
						err = json.Unmarshal(event.GetRawPayload(), &prEvent)
						if err != nil {
							panic(err)
						}

						p.HandlePREvent(&prEvent)
						p.Database.AddEvent(event.GetID())
					}
				}
			}
		}

		log.Debugf("Sleeping for %d as specified by GitHub\n", pollInterval)
		time.Sleep(time.Duration(pollInterval) * time.Second)
	}
}

func (p PRMirror) MirrorPR(pr *github.PullRequest) (int, error) {
	log.Infof("Mirroring PR [%d]: %s from %s\n", pr.GetNumber(), pr.GetTitle(), pr.User.GetLogin())

	base := "master"
	maintainerCanModify := false
	title := fmt.Sprintf("[MIRROR] %s", pr.GetTitle())
	body := fmt.Sprintf("Original PR: %s\n--------------------\n%s", pr.GetHTMLURL(), pr.GetBody())

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

func (p PRMirror) CreateLabel(labelText string, labelColour string) bool {
	label := github.Label{
		Name:  &labelText,
		Color: &labelColour,
	}

	_, _, err := p.GitHubClient.Issues.CreateLabel(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, &label)
	if err != nil {
		log.Errorf("Error while creating a label - %s", err.Error())
		return false
	}

	return true
}

func (p PRMirror) AddLabels(id int, labels []string) bool {
	_, _, err := p.GitHubClient.Issues.AddLabelsToIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while adding a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

func (p PRMirror) RemoveLabel(id int, labels string) bool {
	_, err := p.GitHubClient.Issues.RemoveLabelForIssue(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, labels)
	if err != nil {
		log.Errorf("Error while removing a label on issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}

func (p PRMirror) AddComment(id int, comment string) bool {
	issueComment := github.IssueComment{}
	issueComment.Body = &comment

	_, _, err := p.GitHubClient.Issues.CreateComment(*p.Context, p.Configuration.DownstreamOwner, p.Configuration.DownstreamRepo, id, &issueComment)
	if err != nil {
		log.Errorf("Error while adding a comment to issue#:%d - %s", id, err.Error())
		return false
	}

	return true
}
