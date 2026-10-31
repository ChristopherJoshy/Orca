package orca

import "context"

func (a *API) registerAgentExecution(runID string) context.Context {
	ctx, cancel := context.WithCancel(context.Background())
	a.agentMu.Lock()
	if a.agentCancels == nil {
		a.agentCancels = make(map[string]context.CancelFunc)
	}
	a.agentCancels[runID] = cancel
	a.agentMu.Unlock()
	return ctx
}

func (a *API) cancelAgentExecution(runID string) {
	a.agentMu.Lock()
	cancel := a.agentCancels[runID]
	a.agentMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

func (a *API) finishAgentExecution(runID string) {
	a.agentMu.Lock()
	cancel := a.agentCancels[runID]
	delete(a.agentCancels, runID)
	a.agentMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
