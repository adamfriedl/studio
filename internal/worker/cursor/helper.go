package cursor

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/adamfriedl/studio/internal/worker"
)

// Helper runs the TypeScript cursor-helper CLI (create/resume/send/wait).
type Helper struct {
	// Bin is an executable or `npx tsx path/to/helper.ts`. Empty → auto-discover.
	Bin     string
	Timeout time.Duration
	Env     []string // extra env; CURSOR_API_KEY must be present in process or here
}

type helperOut struct {
	OK      bool   `json:"ok"`
	AgentID string `json:"agentId"`
	RunID   string `json:"runId"`
	Status  string `json:"status"`
	PRURL   string `json:"prURL"`
	Branch  string `json:"branch"`
	Message string `json:"message"`
	Error   string `json:"error"`
}

func (h Helper) timeout() time.Duration {
	if h.Timeout > 0 {
		return h.Timeout
	}
	return 45 * time.Minute
}

func (h Helper) bin() (bin string, prefix []string, dir string, err error) {
	if env := strings.TrimSpace(os.Getenv("STUDIO_CURSOR_HELPER")); env != "" {
		h.Bin = env
	}
	if h.Bin != "" {
		return h.Bin, nil, "", nil
	}
	candidates := []string{"scripts/cursor-helper/helper.ts"}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "scripts/cursor-helper/helper.ts"))
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			abs, _ := filepath.Abs(c)
			return "npx", []string{"--yes", "tsx", abs}, filepath.Dir(abs), nil
		}
	}
	return "", nil, "", fmt.Errorf("cursor helper: helper.ts not found; set STUDIO_CURSOR_HELPER")
}

func (h Helper) run(ctx context.Context, args []string, prompt string) (helperOut, error) {
	ctx, cancel := context.WithTimeout(ctx, h.timeout())
	defer cancel()

	bin, prefix, dir, err := h.bin()
	if err != nil {
		return helperOut{}, err
	}
	cmdArgs := append(append([]string{}, prefix...), args...)
	cmd := exec.CommandContext(ctx, bin, cmdArgs...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), h.Env...)
	if prompt != "" {
		cmd.Stdin = strings.NewReader(prompt)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err = cmd.Run()
	var out helperOut
	if decErr := json.Unmarshal(bytes.TrimSpace(stdout.Bytes()), &out); decErr != nil {
		if err != nil {
			return helperOut{}, fmt.Errorf("cursor helper: %w; stderr=%s stdout=%s", err, stderr.String(), stdout.String())
		}
		return helperOut{}, fmt.Errorf("cursor helper: decode output: %w; stdout=%s", decErr, stdout.String())
	}
	if err != nil && out.Error == "" {
		out.Error = firstNonEmpty(out.Message, err.Error())
	}
	if !out.OK && out.Error == "" {
		out.Error = firstNonEmpty(out.Message, strings.TrimSpace(stderr.String()), "helper failed")
	}
	return out, err
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (h Helper) Ping(ctx context.Context) error {
	out, err := h.run(ctx, []string{"ping"}, "")
	if err != nil {
		return fmt.Errorf("cursor ping: %w (%s)", err, out.Error)
	}
	if !out.OK {
		return fmt.Errorf("cursor ping: %s", out.Error)
	}
	return nil
}

func (h Helper) Start(ctx context.Context, req worker.StartReq) (worker.Result, error) {
	args := []string{"create", "--repo", req.RepoURL, "--prompt-file", "-"}
	if req.StartingRef != "" {
		args = append(args, "--ref", req.StartingRef)
	}
	if req.Model != "" {
		args = append(args, "--model", req.Model)
	}
	if req.AutoCreatePR {
		args = append(args, "--auto-pr")
	}
	out, err := h.run(ctx, args, req.Prompt)
	res := worker.Result{
		AgentID: out.AgentID,
		Status:  out.Status,
		PRURL:   out.PRURL,
		RunID:   out.RunID,
		Branch:  out.Branch,
		Message: out.Message,
	}
	if err != nil {
		if res.Status == "" {
			res.Status = "error"
		}
		detail := firstNonEmpty(out.Error, out.Message)
		return res, fmt.Errorf("cursor start: %w (%s)", err, detail)
	}
	if !out.OK {
		res.Status = "error"
		return res, fmt.Errorf("cursor start: %s", firstNonEmpty(out.Error, out.Message, "unknown"))
	}
	return res, nil
}

func (h Helper) FollowUp(ctx context.Context, req worker.FollowUpReq) (worker.Result, error) {
	args := []string{"followup", "--agent-id", req.AgentID, "--prompt-file", "-"}
	out, err := h.run(ctx, args, req.Prompt)
	res := worker.Result{
		AgentID: out.AgentID,
		Status:  out.Status,
		PRURL:   out.PRURL,
		RunID:   out.RunID,
		Branch:  out.Branch,
		Message: out.Message,
	}
	if err != nil {
		if res.Status == "" {
			res.Status = "error"
		}
		return res, fmt.Errorf("cursor followup: %w (%s)", err, out.Error)
	}
	if !out.OK {
		res.Status = "error"
		return res, fmt.Errorf("cursor followup: %s", out.Error)
	}
	return res, nil
}
