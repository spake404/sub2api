package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// V2 follows the first-turn and independent-follow-up layouts in test2.har.
// A parent thread is always present; parent/root turns exist only when the
// incoming request actually carries a parent-turn relationship. No history is
// loaded and no synthetic parent turn is attached to an independent new turn.
func resolveCodexSubagentV2IDs(c *gin.Context, account *Account, seed string, source *codexSubagentSource) *codexFingerprintIDs {
	ids := &codexFingerprintIDs{
		accountID: account.ID, mode: codexFingerprintSubagentV2,
		installationID:      resolveConvergedInstallationID(account, seed),
		sessionID:           resolveConvergedSessionID(seed),
		threadID:            deriveStableUUIDv4("sub2api:subagent-thread:" + seed + "\x00" + source.threadKey),
		turnStartedAtUnixMs: source.startedAt,
		subagent: &codexSubagentIdentity{
			contextWindow: deriveStableUUIDv4("sub2api:subagent-context-window:" + seed),
		},
	}
	ids.turnID = deriveStableUUIDv4("sub2api:subagent-turn:" + ids.threadID + "\x00" + source.turnID)
	ids.windowID = ids.threadID + ":0"
	if parentTurn := strings.TrimSpace(source.parentTurnID); parentTurn != "" {
		// Sibling threads from the same parent turn must map to the same turn.
		turnNamespace := fmt.Sprintf("sub2api:subagent-parent-turn:%s\x00%d\x00", seed, getAPIKeyIDFromContext(c))
		ids.subagent.parentTurnID = deriveStableUUIDv4(turnNamespace + parentTurn)
		if rootTurn := strings.TrimSpace(source.rootTurnID); rootTurn != "" {
			ids.subagent.rootTurnID = deriveStableUUIDv4(turnNamespace + rootTurn)
		}
	}
	metadata := make(map[string]any, len(source.metadata)+12)
	for key, value := range source.metadata {
		metadata[key] = value
	}
	delete(metadata, "parent_turn_id")
	delete(metadata, "root_turn_id")
	for key, value := range map[string]any{
		"installation_id": ids.installationID, "session_id": ids.sessionID, "thread_id": ids.threadID,
		"parent_thread_id": ids.sessionID, "turn_id": ids.turnID,
		"window_id": ids.windowID, "window_number": 0,
		"context_window_id": ids.subagent.contextWindow, "turn_started_at_unix_ms": ids.turnStartedAtUnixMs,
		"request_kind": "turn", "thread_source": "subagent", "subagent_kind": "thread_spawn",
	} {
		metadata[key] = value
	}
	if ids.subagent.parentTurnID != "" {
		metadata["parent_turn_id"] = ids.subagent.parentTurnID
	}
	if ids.subagent.rootTurnID != "" {
		metadata["root_turn_id"] = ids.subagent.rootTurnID
	}
	encoded, _ := json.Marshal(metadata)
	ids.subagent.turnMetadata = string(encoded)
	logger.FromContext(c.Request.Context()).Info("openai subagent session v2",
		zap.Int64("account_id", account.ID), zap.String("parent_session_id", ids.sessionID),
		zap.String("child_thread_id", ids.threadID), zap.String("turn_id", ids.turnID))
	return ids
}

func applyCodexSubagentV2Headers(h http.Header, ids *codexFingerprintIDs) {
	for name, value := range map[string]string{
		"session-id": ids.sessionID, "thread-id": ids.threadID,
		"x-client-request-id": ids.threadID, "x-codex-parent-thread-id": ids.sessionID,
		"x-codex-window-id": ids.windowID,
		"x-openai-subagent": "collab_spawn", "x-codex-turn-metadata": ids.subagent.turnMetadata,
	} {
		h.Set(name, value)
	}
}

func finalizeCodexSubagentV2Headers(c *gin.Context, account *Account, h http.Header) {
	ids := stagedCodexFingerprintIDs(c, account)
	if ids == nil || ids.mode != codexFingerprintSubagentV2 {
		return
	}
	for _, name := range []string{"session_id", "thread_id", "x-codex-installation-id", openAICodexRoutingHintHeader} {
		deleteOpenAIHeaderEqualFold(h, name)
	}
}

func applyCodexSubagentV2ClientMetadata(metadata map[string]any, ids *codexFingerprintIDs) {
	// These fields belong inside the JSON-encoded turn metadata, not outside.
	for _, name := range []string{
		"parent_thread_id", "window_id", "window_number", "context_window_id",
		"thread_source", "subagent_kind", "request_kind", "parent_turn_id", "root_turn_id",
	} {
		delete(metadata, name)
	}
	for name, value := range map[string]string{
		"session_id": ids.sessionID, "thread_id": ids.threadID, "turn_id": ids.turnID,
		"x-codex-parent-thread-id": ids.sessionID, "x-codex-window-id": ids.windowID,
		"x-openai-subagent": "collab_spawn", "x-codex-installation-id": ids.installationID,
		"x-codex-turn-metadata": ids.subagent.turnMetadata,
	} {
		metadata[name] = value
	}
	if ids.subagent.parentTurnID != "" {
		metadata["parent_turn_id"] = ids.subagent.parentTurnID
	}
	if ids.subagent.rootTurnID != "" {
		metadata["root_turn_id"] = ids.subagent.rootTurnID
	}
}
