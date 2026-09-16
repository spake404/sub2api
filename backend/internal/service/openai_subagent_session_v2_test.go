package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type subagentV2FixtureRequest struct {
	Entry   int               `json:"entry"`
	Child   string            `json:"child"`
	Initial bool              `json:"initial"`
	Headers map[string]string `json:"headers"`
	Body    map[string]any    `json:"body"`
}

func loadSubagentV2Fixture(t *testing.T) []subagentV2FixtureRequest {
	t.Helper()
	raw, err := os.ReadFile("testdata/codex_subagent_v2.json")
	require.NoError(t, err)
	var fixture struct {
		Requests []subagentV2FixtureRequest `json:"requests"`
	}
	require.NoError(t, json.Unmarshal(raw, &fixture))
	require.Len(t, fixture.Requests, 6)
	return fixture.Requests
}

// Exercise the real HTTP builders with the six mocked requests extracted from
// the capture: A/B/C initial calls followed by a separate message to each child.
func TestSubagentV2HTTPForwardingHARSequence(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("passthrough=%v/stream=%v", passthrough, stream), func(t *testing.T) {
				fixtures := loadSubagentV2Fixture(t)
				captured := make(chan subagentCapturedRequest, 16)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), http.StatusBadRequest)
						return
					}
					cm, ok := body["client_metadata"].(map[string]any)
					if !ok {
						http.Error(w, "missing client metadata", http.StatusBadRequest)
						return
					}
					for key, value := range cm {
						if _, ok := value.(string); !ok {
							http.Error(w, "non-string client_metadata."+key, http.StatusBadRequest)
							return
						}
					}
					state := "mock-v2-state-" + r.Header.Get("Thread-Id")
					captured <- subagentCapturedRequest{r.Header.Clone(), body, state}
					w.Header().Set("Content-Type", "text/event-stream")
					w.Header().Set("X-Codex-Turn-State", state)
					fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_mock_v2\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"OK\"}]}],\"usage\":{\"input_tokens\":11,\"output_tokens\":1}}}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				target, err := url.Parse(upstream.URL)
				require.NoError(t, err)
				account := newTestOAuthAccount(86, map[string]any{codexFingerprintModeExtraKey: "subagent_v2", "openai_passthrough": passthrough})
				account.Credentials = map[string]any{"access_token": "mock-oauth-token", "chatgpt_account_id": "mock-account"}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &subagentHTTPTransport{target}}
				router := gin.New()
				router.POST("/v1/responses", func(c *gin.Context) {
					body, err := io.ReadAll(c.Request.Body)
					if err != nil {
						c.AbortWithStatus(http.StatusBadRequest)
						return
					}
					key, _ := strconv.ParseInt(c.GetHeader("X-Test-Key"), 10, 64)
					c.Set("api_key", &APIKey{ID: key})
					SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
					selected := account
					if c.GetHeader("X-Test-Account") == "other" {
						other := *account
						other.ID = 87
						other.Extra = map[string]any{codexFingerprintModeExtraKey: "subagent_v2", "openai_passthrough": passthrough,
							codexFingerprintSeedExtraKey: "22222222-2222-4222-8222-222222222222"}
						selected = &other
					}
					if _, err := svc.Forward(c.Request.Context(), c, selected, body); err != nil {
						t.Errorf("Forward failed: %v", err)
					}
				})
				gateway := httptest.NewServer(router)
				defer gateway.Close()
				send := func(fixture subagentV2FixtureRequest, key int64, state string) subagentCapturedRequest {
					t.Helper()
					fixture.Body["stream"] = stream
					payload, err := json.Marshal(fixture.Body)
					require.NoError(t, err)
					req, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/responses", bytes.NewReader(payload))
					require.NoError(t, err)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("User-Agent", "Codex Desktop/0.153.4")
					req.Header.Set("X-Test-Key", strconv.FormatInt(key, 10))
					for name, value := range fixture.Headers {
						req.Header.Set(name, value)
					}
					if state != "" {
						req.Header.Set("X-Codex-Turn-State", state)
					}
					resp, err := gateway.Client().Do(req)
					require.NoError(t, err)
					content, err := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					require.NoError(t, err)
					require.Equal(t, http.StatusOK, resp.StatusCode, string(content))
					require.Contains(t, string(content), "OK")
					var got subagentCapturedRequest
					select {
					case got = <-captured:
					default:
						t.Fatal("upstream received no request")
					}
					cm := got.body["client_metadata"].(map[string]any)
					var meta, originalMeta map[string]any
					require.NoError(t, json.Unmarshal([]byte(got.headers.Get("X-Codex-Turn-Metadata")), &meta))
					require.NoError(t, json.Unmarshal([]byte(fixture.Headers["x-codex-turn-metadata"]), &originalMeta))
					assert.Equal(t, fixture.Body["input"], got.body["input"], "no parent or foreign child history injected")
					assert.Equal(t, fixture.Body["instructions"], got.body["instructions"])
					assert.Equal(t, got.headers.Get("Session-Id"), got.body["prompt_cache_key"])
					assert.Equal(t, got.headers.Get("Session-Id"), cm["session_id"])
					assert.Equal(t, got.headers.Get("Thread-Id"), cm["thread_id"])
					assert.Equal(t, got.headers.Get("Thread-Id"), got.headers.Get("X-Client-Request-Id"))
					assert.Equal(t, got.headers.Get("Session-Id"), got.headers.Get("X-Codex-Parent-Thread-Id"))
					assert.Equal(t, got.headers.Get("Thread-Id")+":0", got.headers.Get("X-Codex-Window-Id"))
					assert.Equal(t, got.headers.Get("X-Codex-Turn-Metadata"), cm["x-codex-turn-metadata"])
					assert.Equal(t, float64(0), meta["window_number"])
					assert.Equal(t, "subagent", meta["thread_source"])
					assert.Equal(t, "collab_spawn", got.headers.Get("X-OpenAI-Subagent"))
					assert.Equal(t, "remote_compaction_v2", got.headers.Get("X-Codex-Beta-Features"))
					assert.Equal(t, "true", got.headers.Get("X-OpenAI-Internal-Codex-Responses-Lite"))
					assert.Equal(t, len(fixture.Body["client_metadata"].(map[string]any)), len(cm))
					assert.Len(t, meta, len(originalMeta))
					for name, value := range fixture.Body["client_metadata"].(map[string]any) {
						assert.Contains(t, cm, name, "same outer fields as HAR")
						assert.IsType(t, value, cm[name])
					}
					for name, value := range originalMeta {
						assert.Contains(t, meta, name)
						assert.IsType(t, value, meta[name])
					}
					for _, name := range []string{"session_id", "thread_id", "x-codex-installation-id", "x-codex-routing-hint"} {
						assert.Empty(t, got.headers.Get(name), name)
					}
					return got
				}
				first := make(map[string]subagentCapturedRequest)
				for _, fixture := range fixtures {
					got := send(fixture, 1, "")
					cm := got.body["client_metadata"].(map[string]any)
					if fixture.Initial {
						first[fixture.Child] = got
						assert.NotEmpty(t, cm["parent_turn_id"])
						assert.Equal(t, cm["parent_turn_id"], cm["root_turn_id"])
					} else {
						previous := first[fixture.Child]
						assert.Equal(t, previous.headers.Get("Thread-Id"), got.headers.Get("Thread-Id"))
						assert.NotEqual(t, previous.body["client_metadata"].(map[string]any)["turn_id"], cm["turn_id"])
						assert.NotContains(t, cm, "parent_turn_id")
						assert.NotContains(t, cm, "root_turn_id")
					}
				}
				for _, child := range []string{"B", "C"} {
					assert.Equal(t, first["A"].headers.Get("Session-Id"), first[child].headers.Get("Session-Id"))
					assert.NotEqual(t, first["A"].headers.Get("Thread-Id"), first[child].headers.Get("Thread-Id"))
					assert.Equal(t, first["A"].body["client_metadata"].(map[string]any)["parent_turn_id"], first[child].body["client_metadata"].(map[string]any)["parent_turn_id"])
				}
				// Tool calls within the original turn retain that turn's parent links.
				tool := fixtures[0]
				tool.Body["input"] = append(tool.Body["input"].([]any),
					map[string]any{"type": "function_call", "call_id": "fc_mock_call", "name": "mock", "arguments": "{}"},
					map[string]any{"type": "function_call_output", "call_id": "fc_mock_call", "output": "A-only"})
				got := send(tool, 1, first["A"].state)
				assert.Equal(t, first["A"].headers.Get("X-Codex-Turn-Metadata"), got.headers.Get("X-Codex-Turn-Metadata"))
				assert.Equal(t, first["A"].state, got.headers.Get("X-Codex-Turn-State"))
				foreign := send(fixtures[1], 1, first["A"].state)
				assert.Empty(t, foreign.headers.Get("X-Codex-Turn-State"))
				otherKey := send(fixtures[0], 2, first["A"].state)
				assert.NotEqual(t, first["A"].headers.Get("Thread-Id"), otherKey.headers.Get("Thread-Id"))
				assert.Empty(t, otherKey.headers.Get("X-Codex-Turn-State"))
				switched := loadSubagentV2Fixture(t)[0]
				switched.Headers["X-Test-Account"] = "other"
				otherAccount := send(switched, 1, first["A"].state)
				assert.NotEqual(t, first["A"].headers.Get("Session-Id"), otherAccount.headers.Get("Session-Id"))
				assert.NotEqual(t, first["A"].headers.Get("Thread-Id"), otherAccount.headers.Get("Thread-Id"))
				assert.Empty(t, otherAccount.headers.Get("X-Codex-Turn-State"))
				// Deterministic identities survive a new process instance.
				svc = &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &subagentHTTPTransport{target}}
				restarted := send(fixtures[0], 1, "")
				assert.Equal(t, first["A"].headers.Get("Thread-Id"), restarted.headers.Get("Thread-Id"))
				assert.False(t, strings.Contains(restarted.headers.Get("X-Codex-Turn-Metadata"), fixtures[0].Headers["thread-id"]))
			})
		}
	}
}

func TestSubagentV2ModeScopeAndIndependentTurns(t *testing.T) {
	created := prepareCodexFingerprintExtraForCreate(PlatformOpenAI, AccountTypeOAuth,
		map[string]any{codexFingerprintModeExtraKey: "subagent_v2"})
	_, ok := codexFingerprintSeed(created)
	require.True(t, ok)
	account := newTestOAuthAccount(86, created)
	assert.Equal(t, codexFingerprintSubagentV2, account.GetCodexFingerprintMode())
	assert.True(t, ShouldEnsureCodexFingerprintSeedForExtraUpdates(map[string]any{codexFingerprintModeExtraKey: "subagent_v2"}))
	body := []byte(`{"client_metadata":{"thread_id":"task-A","turn_id":"turn-2","root_turn_id":"own-new-turn"}}`)
	c := subagentTestContext(body, "task-A", 1)
	ids := resolveCodexHTTPFingerprintIDs(c, account, body)
	require.NotNil(t, ids)
	var meta map[string]any
	require.NoError(t, json.Unmarshal([]byte(ids.subagent.turnMetadata), &meta))
	assert.NotContains(t, meta, "parent_turn_id")
	assert.NotContains(t, meta, "root_turn_id", "an independent user turn has no parent turn")
	assert.Equal(t, ids.threadID, resolveCodexHTTPFingerprintIDs(c, account, body).threadID)
	assert.Equal(t, ids.turnID, resolveCodexHTTPFingerprintIDs(c, account, body).turnID)
	for _, path := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses/compact"} {
		c.Request.URL.Path = path
		assert.Nil(t, resolveCodexHTTPFingerprintIDs(c, account, body))
	}
	c.Request.URL.Path = "/v1/responses"
	SetOpenAIClientTransport(c, OpenAIClientTransportWS)
	assert.Nil(t, resolveCodexHTTPFingerprintIDs(c, account, body))
	assert.Equal(t, codexFingerprintOff, activeCodexFingerprintMode(account))
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	account.Type = AccountTypeAPIKey
	assert.Nil(t, resolveCodexHTTPFingerprintIDs(c, account, body))
}
