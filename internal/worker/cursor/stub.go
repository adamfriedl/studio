package cursor

import (
	"context"
	"fmt"

	"github.com/adamfriedl/studio/internal/worker"
)

// Stub until Phase 1 Cursor cloud spike lands a real client.
type Stub struct{}

func (Stub) Start(context.Context, worker.StartReq) (worker.Result, error) {
	return worker.Result{}, fmt.Errorf("cursor worker: not implemented")
}

func (Stub) FollowUp(context.Context, worker.FollowUpReq) (worker.Result, error) {
	return worker.Result{}, fmt.Errorf("cursor worker: not implemented")
}

func (Stub) Ping(context.Context) error {
	return fmt.Errorf("cursor worker: not implemented")
}
