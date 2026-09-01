package core

import (
	"context"
	"iter"

	"github.com/genai-io/sdk-go/pkg/ai"
)

// Faking the model now means faking the protocol, which is the seam pkg/ai
// already has and the one five real drivers sit behind. A test says what the
// endpoint sends; nothing in between has to be stubbed.

// deltas renders one answer as the stream a driver would produce.
func deltas(r InferResponse) []ai.Delta {
	var out []ai.Delta
	if r.Thinking != "" {
		out = append(out, ai.Delta{Block: ai.ThinkingBlock(r.Thinking, r.ThinkingSignature)}, ai.Delta{EndBlock: true})
	}
	if r.Content != "" {
		out = append(out, ai.Delta{Block: ai.TextBlock(r.Content)}, ai.Delta{EndBlock: true})
	}
	for _, c := range r.ToolCalls {
		out = append(out, ai.Delta{Block: ai.ToolCallBlock(ai.ToolCall{
			ID: c.ID, Name: c.Name, Input: c.Input, Signature: c.ThoughtSignature,
		})})
	}
	stop := ai.StopEndTurn
	switch r.StopReason {
	case StopMaxTokens:
		stop = ai.StopMaxTokens
	case StopToolUse:
		stop = ai.StopToolUse
	}
	if len(r.ToolCalls) > 0 {
		stop = ai.StopToolUse
	}
	out = append(out, ai.Delta{
		StopReason: stop,
		Usage: &ai.Usage{
			Input: r.InputTokens, Output: r.OutputTokens,
			CacheWrite: r.CacheCreationInputTokens, CacheRead: r.CacheReadInputTokens,
		},
	})
	return out
}

// scriptedDriver answers each call with the next script, then repeats the last.
type scriptedDriver struct {
	scripts [][]ai.Delta
	errs    []error
	calls   int
	// before runs on each call, for a test that needs to observe or block.
	before func(ctx context.Context, n int)
}

func (d *scriptedDriver) Name() string { return "scripted" }

func (d *scriptedDriver) Stream(ctx context.Context, _ *ai.Request) iter.Seq2[ai.Delta, error] {
	n := d.calls
	d.calls++
	return func(yield func(ai.Delta, error) bool) {
		if d.before != nil {
			d.before(ctx, n)
		}
		if n < len(d.scripts) {
			for _, delta := range d.scripts[n] {
				if !yield(delta, nil) {
					return
				}
			}
		}
		if n < len(d.errs) && d.errs[n] != nil {
			yield(ai.Delta{}, d.errs[n])
		}
	}
}

// testClient wraps a driver as the client an agent is built on.
func testClient(d ai.Driver) *ai.Client {
	return ai.NewClientWithDriver(d, ai.Model{ID: "stub", API: "stub", ContextWindow: 200_000})
}
