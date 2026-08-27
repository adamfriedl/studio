package run

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/adamfriedl/studio/internal/binding"
	"github.com/adamfriedl/studio/internal/worker"
)

type mockWorker struct {
	followErr error
	followRes worker.Result
	startErr  error
	startRes  worker.Result
	followN   int
	startN    int
	lastStart worker.StartReq
}

func (m *mockWorker) FollowUp(ctx context.Context, req worker.FollowUpReq) (worker.Result, error) {
	m.followN++
	return m.followRes, m.followErr
}

func (m *mockWorker) Start(ctx context.Context, req worker.StartReq) (worker.Result, error) {
	m.startN++
	m.lastStart = req
	return m.startRes, m.startErr
}

func TestSendOrReplaceFollowUpOK(t *testing.T) {
	w := &mockWorker{followRes: worker.Result{AgentID: "bc-old", RunID: "r1", Status: "finished"}}
	b := &binding.Binding{AgentID: "bc-old", Repo: "o/r", Branch: "studio/1-x", PRURL: "https://github.com/o/r/pull/1"}
	res, out, replaced, err := SendOrReplace(context.Background(), w, b, "fix it", "composer-2.5", allowAll)
	if err != nil || replaced || res.RunID != "r1" || out.AgentID != "bc-old" || w.startN != 0 {
		t.Fatalf("got res=%#v out=%#v replaced=%v err=%v startN=%d", res, out, replaced, err, w.startN)
	}
}

func allowAll(string) bool { return true }

func TestSendOrReplaceResumeFailThenReplace(t *testing.T) {
	w := &mockWorker{
		followErr: errors.New("resume gone"),
		startRes:  worker.Result{AgentID: "bc-new", RunID: "r2", Status: "finished"},
	}
	b := &binding.Binding{AgentID: "bc-old", Repo: "adamfriedl/pad-lab", Branch: "studio/8-fix", PRURL: "https://github.com/adamfriedl/pad-lab/pull/1"}
	res, out, replaced, err := SendOrReplace(context.Background(), w, b, "CI logs…", "composer-2.5", allowAll)
	if err != nil || !replaced || res.AgentID != "bc-new" || out.AgentID != "bc-new" {
		t.Fatalf("got res=%#v out=%#v replaced=%v err=%v", res, out, replaced, err)
	}
	if w.startN != 1 || w.followN != 1 {
		t.Fatalf("calls follow=%d start=%d", w.followN, w.startN)
	}
	if w.lastStart.StartingRef != "studio/8-fix" || w.lastStart.AutoCreatePR {
		t.Fatalf("start req %#v", w.lastStart)
	}
	if !strings.Contains(w.lastStart.Prompt, "could not be resumed") || !strings.Contains(w.lastStart.Prompt, "CI logs…") {
		t.Fatalf("prompt: %s", w.lastStart.Prompt)
	}
	if !strings.HasPrefix(w.lastStart.RepoURL, "https://github.com/") {
		t.Fatalf("repo url %q", w.lastStart.RepoURL)
	}
}

func TestSendOrReplaceMissingAgentID(t *testing.T) {
	w := &mockWorker{startRes: worker.Result{AgentID: "bc-new"}}
	b := &binding.Binding{Repo: "o/r", Branch: "studio/1-x", PRURL: "https://github.com/o/r/pull/1"}
	_, out, replaced, err := SendOrReplace(context.Background(), w, b, "review…", "", allowAll)
	if err != nil || !replaced || out.AgentID != "bc-new" || w.followN != 0 {
		t.Fatalf("got out=%#v replaced=%v err=%v followN=%d", out, replaced, err, w.followN)
	}
}

func TestSendOrReplaceCannotWithoutBranch(t *testing.T) {
	w := &mockWorker{followErr: errors.New("nope")}
	b := &binding.Binding{AgentID: "bc-old", Repo: "o/r"}
	_, _, _, err := SendOrReplace(context.Background(), w, b, "x", "", allowAll)
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestSendOrReplaceStartFails(t *testing.T) {
	w := &mockWorker{
		followErr: errors.New("resume gone"),
		startErr:  errors.New("create failed"),
	}
	b := &binding.Binding{AgentID: "bc-old", Repo: "o/r", Branch: "studio/1-x"}
	_, out, replaced, err := SendOrReplace(context.Background(), w, b, "x", "", allowAll)
	if err == nil || !replaced || out.AgentID != "" {
		t.Fatalf("got out=%#v replaced=%v err=%v", out, replaced, err)
	}
}

func TestSendOrReplaceRejectsNonAllowlisted(t *testing.T) {
	w := &mockWorker{followErr: errors.New("resume gone")}
	b := &binding.Binding{AgentID: "bc-old", Repo: "evil/x", Branch: "studio/1-x"}
	_, _, _, err := SendOrReplace(context.Background(), w, b, "x", "", func(string) bool { return false })
	if err == nil || w.startN != 0 {
		t.Fatalf("expected allowlist error, startN=%d err=%v", w.startN, err)
	}
}
