package llm

import (
	"github.com/genai-io/sdk-go/pkg/ai"

	"github.com/genai-io/san/internal/core"
)

// Translating between San's conversation types and the SDK's.
//
// The two disagree on one thing only, and it is the whole of this file: San
// holds a turn as parallel fields — Content beside Thinking beside ToolCalls —
// while the SDK holds it as the ordered block sequence the provider produced.
// Going out, the fields are laid down in the order every protocol wants them
// replayed: reasoning first, then the answer, then the calls it asked for.
// Coming back, the blocks are projected onto the fields San's UI reads.

// toModelInfo projects an SDK model onto the record San's picker and store
// read. A model with no window reports zero, which San already treats as
// "unknown" rather than substituting a guess.
func toModelInfo(m ai.Model) ModelInfo {
	info := ModelInfo{
		ID:               m.ID,
		Name:             m.Name,
		DisplayName:      m.Name,
		InputTokenLimit:  m.ContextWindow,
		OutputTokenLimit: m.MaxOutput,
	}
	if info.Name == "" {
		info.Name = m.ID
		info.DisplayName = m.ID
	}
	if efforts := m.Efforts(); len(efforts) > 0 {
		labels := make([]string, len(efforts))
		for i, effort := range efforts {
			labels[i] = string(effort)
		}
		var fallback string
		if level, ok := m.DefaultLevel(); ok {
			fallback = string(level.Effort)
		}
		info.Reasoning = NewReasoningCapability(labels, fallback)
	}
	return info
}

// ToAIMessages and ToAITools are the conversions, exported for the one caller
// outside this package that builds a request itself.
func ToAIMessages(msgs []core.Message, model ai.Model) []ai.Message {
	return core.ToAIMessages(msgs, model)
}

// ToAITools converts San's schemas into the SDK's tools.
func ToAITools(schemas []ToolSchema) []ai.Tool { return core.ToAITools(schemas) }
