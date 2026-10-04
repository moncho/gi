package inference

import (
	"context"
	"fmt"

	"github.com/rcarmo/gi/internal/compaction"
	"github.com/rcarmo/gi/internal/store"
	goai "github.com/rcarmo/go-ai"
)

// SessionContextUsage exposes only measured values. Nil fields remain unknown.
// Capacity is catalogue metadata; it is not an invented token measurement.
func SessionContextUsage(ctx context.Context, s *store.Store, sessionID string, window int) (map[string]any, error) {
	measurement, err := s.LatestContextMeasurement(ctx, sessionID)
	if err != nil {
		return nil, err
	}
	result := map[string]any{"tokens": nil, "contextWindow": nil, "percent": nil, "source": "unavailable", "measurement": nil}
	if window > 0 {
		result["contextWindow"] = window
	}
	stale := false
	if measurement != nil {
		if stale, err = s.ContextMeasurementStale(ctx, sessionID); err != nil {
			return nil, err
		}
	}
	if stale {
		// After compaction the measured request no longer describes the context.
		// Like Piclaw, show the local estimate of the compacted context, marked as such.
		tokens, err := estimateSessionContext(ctx, s, sessionID)
		if err != nil {
			return nil, err
		}
		result["tokens"] = tokens
		result["source"] = "estimate"
		if window > 0 {
			result["percent"] = float64(tokens) * 100 / float64(window)
		}
	} else if measurement != nil {
		result["tokens"] = measurement.Tokens
		result["source"] = "provider_request"
		result["measurement"] = measurement
		if window > 0 {
			result["percent"] = float64(measurement.Tokens) * 100 / float64(window)
		}
	}
	return result, nil
}

func CheckSessionModelContext(ctx context.Context, s *store.Store, sessionID string, option ModelOption) error {
	if option.ContextWindow <= 0 {
		return nil
	}
	measurement, err := s.LatestContextMeasurement(ctx, sessionID)
	if err != nil {
		return err
	}
	if measurement != nil && measurement.Tokens > option.ContextWindow {
		return fmt.Errorf("model %q context window %d is smaller than latest measured request (%d tokens)", option.Label, option.ContextWindow, measurement.Tokens)
	}
	return nil
}

func estimateSessionContext(ctx context.Context, s *store.Store, sessionID string) (int, error) {
	snapshot, err := s.ContextSnapshot(ctx, sessionID)
	if err != nil {
		return 0, err
	}
	var messages []goai.Message
	if snapshot.Summary != "" {
		messages = append(messages, goai.UserMessage(compaction.SummaryPrefix+snapshot.Summary+compaction.SummarySuffix))
	}
	for _, m := range snapshot.Messages {
		messages = append(messages, goai.Message{Role: goai.Role(m.Role), Content: []goai.ContentBlock{{Type: "text", Text: m.Content}}})
	}
	return compaction.EstimateMessagesTokens(messages), nil
}
