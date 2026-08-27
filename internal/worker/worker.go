package worker

import "context"

type StartReq struct {
	IssueNumber  int
	Title        string
	Body         string
	RepoURL      string
	StartingRef  string
	BranchName   string
	Model        string
	AutoCreatePR bool
	Prompt       string
}

type FollowUpReq struct {
	AgentID string
	Prompt  string
}

type Result struct {
	AgentID  string
	AgentURL string
	Status   string // finished | error | running | ...
	PRURL    string
	RunID    string
	Branch   string
	Message  string
}

type Worker interface {
	Start(ctx context.Context, req StartReq) (Result, error)
	FollowUp(ctx context.Context, req FollowUpReq) (Result, error)
	Ping(ctx context.Context) error
}
