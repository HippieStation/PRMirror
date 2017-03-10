package main

import "github.com/google/go-github/github"
import "fmt"

type MirroredPR struct {
	DownstreamID int
	UpstreamID   int
}

func MirrorPR(PREvent *github.PullRequestEvent) {
	fmt.Printf("Mirroring new PR: %s\n", PREvent.PullRequest.GetTitle())
}
