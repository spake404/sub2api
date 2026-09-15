package service

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const codexSubagentSourceContextKey = "codex_subagent_source"

// Kept before any account-specific rewrite, including retries onto another account.
type codexSubagentSource struct {
	threadKey    string
	turnID       string
	parentTurnID string
	rootTurnID   string
	startedAt    int64
	metadata     map[string]any
}

type codexSubagentIdentity struct {
	rootTurnID    string
	parentTurnID  string
	contextWindow string
	turnMetadata  string
}

func isCodexSubagentRequest(c *gin.Context, account *Account) bool {
	return account != nil && isCodexSubagentMode(account.GetCodexFingerprintMode()) &&
		c != nil && c.Request != nil && c.Request.Method == http.MethodPost &&
		c.Request.URL.Path == "/v1/responses" && GetOpenAIClientTransport(c) != OpenAIClientTransportWS &&
		!strings.EqualFold(c.GetHeader("Upgrade"), "websocket")
}

func codexMetadataObject(raw string) map[string]any {
	var value map[string]any
	if json.Unmarshal([]byte(raw), &value) != nil || value == nil {
		return map[string]any{}
	}
	return value
}

func captureCodexSubagentSource(c *gin.Context, body []byte) *codexSubagentSource {
	if value, ok := c.Get(codexSubagentSourceContextKey); ok {
		return value.(*codexSubagentSource)
	}
	cm := gjson.GetBytes(body, "client_metadata")
	metadata := codexMetadataObject(cm.Get("x-codex-turn-metadata").String())
	headerMetadata := codexMetadataObject(c.GetHeader("X-Codex-Turn-Metadata"))
	for key, value := range headerMetadata {
		metadata[key] = value
	}
	source := &codexSubagentSource{metadata: metadata, startedAt: time.Now().UnixMilli()}
	source.parentTurnID, _ = metadata["parent_turn_id"].(string)
	if source.parentTurnID == "" {
		source.parentTurnID = cm.Get("parent_turn_id").String()
	}
	source.rootTurnID, _ = metadata["root_turn_id"].(string)
	if source.rootTurnID == "" {
		source.rootTurnID = cm.Get("root_turn_id").String()
	}
	// Prefer a thread identifier from any supported carrier before a session identifier.
	for _, field := range []string{"thread", "session"} {
		for _, value := range []string{
			c.GetHeader(field + "-id"), c.GetHeader(field + "_id"),
			cm.Get(field + "_id").String(), gjson.Get(c.GetHeader("X-Codex-Turn-Metadata"), field+"_id").String(),
			gjson.Get(cm.Get("x-codex-turn-metadata").String(), field+"_id").String(),
		} {
			if value = strings.TrimSpace(value); value != "" {
				source.threadKey = field + ":" + value
				break
			}
		}
		if source.threadKey != "" {
			break
		}
	}
	if source.threadKey == "" {
		source.threadKey = "request:" + uuid.NewString()
	}
	source.threadKey = fmt.Sprintf("%d\x00%s", getAPIKeyIDFromContext(c), source.threadKey)
	source.turnID = strings.TrimSpace(cm.Get("turn_id").String())
	if source.turnID == "" {
		source.turnID, _ = metadata["turn_id"].(string)
		source.turnID = strings.TrimSpace(source.turnID)
	}
	if source.turnID == "" {
		source.turnID = uuid.NewString()
	}
	if started, ok := metadata["turn_started_at_unix_ms"].(float64); ok && started > 0 {
		source.startedAt = int64(started)
	}
	c.Set(codexSubagentSourceContextKey, source)
	return source
}

func resolveCodexHTTPFingerprintIDs(c *gin.Context, account *Account, originalBody []byte) *codexFingerprintIDs {
	if account == nil {
		return nil
	}
	if !isCodexSubagentMode(account.GetCodexFingerprintMode()) {
		var headers http.Header
		if c != nil && c.Request != nil {
			headers = c.Request.Header
		}
		return resolveCodexFingerprintIDsFromRequest(account, headers)
	}
	if !isCodexSubagentRequest(c, account) {
		return nil
	}
	seed, ok := codexFingerprintSeed(account.Extra)
	if !ok {
		return nil
	}
	source := captureCodexSubagentSource(c, originalBody)
	if account.GetCodexFingerprintMode() == codexFingerprintSubagentV2 {
		return resolveCodexSubagentV2IDs(c, account, seed, source)
	}
	ids := &codexFingerprintIDs{
		accountID: account.ID, mode: codexFingerprintSubagent,
		installationID:      resolveConvergedInstallationID(account, seed),
		sessionID:           resolveConvergedSessionID(seed),
		threadID:            deriveStableUUIDv4("sub2api:subagent-thread:" + seed + "\x00" + source.threadKey),
		turnStartedAtUnixMs: source.startedAt,
		subagent: &codexSubagentIdentity{
			rootTurnID:    deriveStableUUIDv4("sub2api:subagent-root-turn:" + seed),
			contextWindow: deriveStableUUIDv4("sub2api:subagent-context-window:" + seed),
		},
	}
	ids.turnID = deriveStableUUIDv4("sub2api:subagent-turn:" + ids.threadID + "\x00" + source.turnID)
	ids.windowID = ids.threadID + ":0"
	metadata := make(map[string]any, len(source.metadata)+15)
	for key, value := range source.metadata {
		metadata[key] = value
	}
	for key, value := range map[string]any{
		"installation_id": ids.installationID, "session_id": ids.sessionID, "thread_id": ids.threadID,
		"parent_thread_id": ids.sessionID, "parent_turn_id": ids.subagent.rootTurnID, "root_turn_id": ids.subagent.rootTurnID,
		"turn_id": ids.turnID, "window_id": ids.windowID, "window_number": 0,
		"context_window_id": ids.subagent.contextWindow, "turn_started_at_unix_ms": ids.turnStartedAtUnixMs,
		"request_kind": "turn", "thread_source": "subagent", "subagent_kind": "thread_spawn",
	} {
		metadata[key] = value
	}
	encoded, _ := json.Marshal(metadata)
	ids.subagent.turnMetadata = string(encoded)
	logger.FromContext(c.Request.Context()).Info("openai subagent session",
		zap.Int64("account_id", account.ID), zap.String("parent_session_id", ids.sessionID),
		zap.String("child_thread_id", ids.threadID), zap.String("turn_id", ids.turnID))
	return ids
}

func applyCodexSubagentHeaders(h http.Header, ids *codexFingerprintIDs) {
	for name, value := range map[string]string{
		"session-id": ids.sessionID, "session_id": ids.sessionID,
		"thread-id": ids.threadID, "thread_id": ids.threadID,
		"x-client-request-id": ids.threadID, "x-codex-parent-thread-id": ids.sessionID,
		"x-codex-window-id": ids.windowID, "x-codex-installation-id": ids.installationID,
		"x-openai-subagent": "collab_spawn", "x-codex-turn-metadata": ids.subagent.turnMetadata,
	} {
		h.Set(name, value)
	}
}

func applyCodexSubagentClientMetadata(metadata map[string]any, ids *codexFingerprintIDs) {
	// Unlike the JSON inside x-codex-turn-metadata, outer values must be strings.
	for name, value := range map[string]string{
		"session_id": ids.sessionID, "thread_id": ids.threadID,
		"parent_thread_id": ids.sessionID, "x-codex-parent-thread-id": ids.sessionID,
		"parent_turn_id": ids.subagent.rootTurnID, "root_turn_id": ids.subagent.rootTurnID,
		"turn_id": ids.turnID, "x-codex-window-id": ids.windowID, "window_id": ids.windowID,
		"window_number": "0", "context_window_id": ids.subagent.contextWindow,
		"x-codex-installation-id": ids.installationID, "x-openai-subagent": "collab_spawn",
		"thread_source": "subagent", "subagent_kind": "thread_spawn", "request_kind": "turn",
		"x-codex-turn-metadata": ids.subagent.turnMetadata,
	} {
		metadata[name] = value
	}
}
