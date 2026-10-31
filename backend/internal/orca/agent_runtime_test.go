package orca

import (
	"context"
	"testing"
)

func TestAgentCancellationStopsInFlightExecutionContext(t *testing.T) {
	api := &API{}
	ctx := api.registerAgentExecution("run-cancel")
	api.cancelAgentExecution("run-cancel")
	if ctx.Err() != context.Canceled {
		t.Fatalf("agent context error = %v, want canceled", ctx.Err())
	}
	api.finishAgentExecution("run-cancel")
	if len(api.agentCancels) != 0 {
		t.Fatalf("cancelled agent execution leaked its context: %#v", api.agentCancels)
	}
}
