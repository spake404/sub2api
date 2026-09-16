package service

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func subagentTestContext(body []byte, thread string, apiKey int64) *gin.Context {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodPost, "/v1/responses", bytes.NewReader(body))
	c.Request.Header.Set("Session-Id", "mock-session")
	if thread != "" {
		c.Request.Header.Set("Thread-Id", thread)
	}
	c.Set("api_key", &APIKey{ID: apiKey})
	SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
	return c
}

func TestSubagentSessionStableThreadsAndTurns(t *testing.T) {
	account := newTestOAuthAccount(86, map[string]any{codexFingerprintModeExtraKey: "subagent"})
	body := []byte(`{"client_metadata":{"turn_id":"turn-1"},"prompt_cache_key":"unchanged"}`)
	c := subagentTestContext(body, "main", 1)
	first := resolveCodexHTTPFingerprintIDs(c, account, body)
	require.NotNil(t, first)
	assert.Equal(t, first, resolveCodexHTTPFingerprintIDs(c, account, body), "internal retry")
	continued := resolveCodexHTTPFingerprintIDs(subagentTestContext(body, "main", 1), account, body)
	assert.Equal(t, first.threadID, continued.threadID)
	assert.Equal(t, first.turnID, continued.turnID, "same turn across separate HTTP requests")
	newTurn := resolveCodexHTTPFingerprintIDs(subagentTestContext(nil, "main", 1), account, []byte(`{"client_metadata":{"turn_id":"turn-2"}}`))
	assert.Equal(t, first.threadID, newTurn.threadID)
	assert.NotEqual(t, first.turnID, newTurn.turnID)
	child := resolveCodexHTTPFingerprintIDs(subagentTestContext(body, "native-child", 1), account, body)
	assert.Equal(t, first.sessionID, child.sessionID)
	assert.NotEqual(t, first.threadID, child.threadID, "HAR parent and child share session but not thread")
	otherUser := resolveCodexHTTPFingerprintIDs(subagentTestContext(body, "main", 2), account, body)
	assert.NotEqual(t, first.threadID, otherUser.threadID)
	assert.Equal(t, first.sessionID, otherUser.sessionID)
	otherAccount := newTestOAuthAccount(87, map[string]any{codexFingerprintModeExtraKey: "subagent", codexFingerprintSeedExtraKey: "22222222-2222-4222-8222-222222222222"})
	moved := resolveCodexHTTPFingerprintIDs(c, otherAccount, body)
	assert.NotEqual(t, first.threadID, moved.threadID)
	assert.NotEqual(t, first.sessionID, moved.sessionID)
	assert.Equal(t, first.threadID, resolveCodexHTTPFingerprintIDs(c, account, body).threadID)
	for _, path := range []string{"/v1/messages", "/v1/chat/completions", "/v1/responses/compact"} {
		scoped := subagentTestContext(body, "main", 1)
		scoped.Request.URL.Path = path
		assert.Nil(t, resolveCodexHTTPFingerprintIDs(scoped, account, body), path)
	}
	SetOpenAIClientTransport(c, OpenAIClientTransportWS)
	assert.Nil(t, resolveCodexHTTPFingerprintIDs(c, account, body))
	assert.Nil(t, resolveCodexFingerprintIDsFromRequest(account, c.Request.Header))
	assert.Equal(t, codexFingerprintOff, activeCodexFingerprintMode(account))
	account.Type = AccountTypeAPIKey
	assert.Nil(t, resolveCodexHTTPFingerprintIDs(subagentTestContext(body, "main", 1), account, body))
}

func TestSubagentSessionMetadataSourcesAndAnonymousRequests(t *testing.T) {
	account := newTestOAuthAccount(86, map[string]any{codexFingerprintModeExtraKey: "subagent"})
	for _, body := range []string{
		`{"client_metadata":{"thread_id":"from-body","turn_id":"turn"}}`,
		`{"client_metadata":{"x-codex-turn-metadata":"{\"thread_id\":\"from-body\",\"turn_id\":\"turn\"}"}}`,
	} {
		c := subagentTestContext(nil, "", 1)
		fromBody := resolveCodexHTTPFingerprintIDs(c, account, []byte(body))
		fromHeader := resolveCodexHTTPFingerprintIDs(subagentTestContext(nil, "from-body", 1), account, []byte(body))
		assert.Equal(t, fromHeader.threadID, fromBody.threadID, "body thread takes priority over header session")
		assert.Equal(t, fromHeader.turnID, fromBody.turnID)
	}
	c := subagentTestContext(nil, "", 1)
	c.Request.Header.Del("Session-Id")
	first := resolveCodexHTTPFingerprintIDs(c, account, []byte(`{}`))
	assert.Equal(t, first, resolveCodexHTTPFingerprintIDs(c, account, []byte(`{}`)))
	next := subagentTestContext(nil, "", 1)
	next.Request.Header.Del("Session-Id")
	assert.NotEqual(t, first.threadID, resolveCodexHTTPFingerprintIDs(next, account, []byte(`{}`)).threadID)
	for _, cm := range []string{`null`, `"invalid"`, `{}`, `{"x-codex-turn-metadata":"not-json"}`} {
		body := []byte(`{"prompt_cache_key":"do-not-rewrite","client_metadata":` + cm + `,"input":[]}`)
		ids := resolveCodexHTTPFingerprintIDs(subagentTestContext(body, "main", 1), account, body)
		raw, changed, err := applyCodexFingerprintClientMetadataRaw(body, ids)
		require.NoError(t, err)
		require.True(t, changed)
		var parsed map[string]any
		require.NoError(t, json.Unmarshal(raw, &parsed))
		assert.Equal(t, "do-not-rewrite", parsed["prompt_cache_key"])
		h := http.Header{}
		applyCodexFingerprintHeaders(h, ids)
		cm, _ := parsed["client_metadata"].(map[string]any)
		assert.Equal(t, h.Get("X-Codex-Turn-Metadata"), cm["x-codex-turn-metadata"])
	}
	created := prepareCodexFingerprintExtraForCreate(PlatformOpenAI, AccountTypeOAuth, map[string]any{codexFingerprintModeExtraKey: "subagent"})
	_, ok := codexFingerprintSeed(created)
	require.True(t, ok)
	assert.True(t, ShouldEnsureCodexFingerprintSeedForExtraUpdates(map[string]any{codexFingerprintModeExtraKey: "subagent"}))
}

func TestSubagentSessionTurnStateAccountSwitch(t *testing.T) {
	account := newTestOAuthAccount(86, map[string]any{codexFingerprintModeExtraKey: "subagent"})
	c := subagentTestContext(nil, "main", 1)
	stageCodexFingerprintIDs(c, resolveCodexHTTPFingerprintIDs(c, account, []byte(`{}`)))
	svc := &OpenAIGatewayService{}
	svc.noteOpenAICodexTurnStateProvenance(c, account, "mock-state")
	for _, next := range []*Account{
		newTestOAuthAccount(87, map[string]any{codexFingerprintModeExtraKey: "subagent"}),
		{ID: 88, Platform: PlatformOpenAI, Type: AccountTypeAPIKey},
	} {
		// An abandoned attempt can leave its staged IDs in the request context.
		// The next account must not use that attempt's provenance key.
		assert.NotEqual(t, codexTurnStateKey(c, account), codexTurnStateKey(c, next))
	}
	other := newTestOAuthAccount(87, map[string]any{codexFingerprintModeExtraKey: "subagent"})
	h := http.Header{}
	h.Set(openAICodexTurnStateHeader, "mock-state")
	svc.guardOpenAICodexTurnStateEcho(c, other, h)
	assert.Empty(t, h.Get(openAICodexTurnStateHeader))
}

type subagentHTTPTransport struct{ target *url.URL }

func (u *subagentHTTPTransport) Do(req *http.Request, _ string, _ int64, _ int) (*http.Response, error) {
	copy := req.Clone(req.Context())
	copy.URL.Scheme, copy.URL.Host = u.target.Scheme, u.target.Host
	return http.DefaultClient.Do(copy)
}

func (u *subagentHTTPTransport) DoWithTLS(req *http.Request, proxy string, id int64, concurrency int, _ *tlsfingerprint.Profile) (*http.Response, error) {
	return u.Do(req, proxy, id, concurrency)
}

type subagentCapturedRequest struct {
	headers http.Header
	body    map[string]any
	state   string
}

// Real HTTP on both sides: client -> Gin/Forward -> local upstream receiver.
// The fixture keeps HAR 4/7/8's identity relationships, not their prompts/credentials.
func TestSubagentSessionHTTPForwarding(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, passthrough := range []bool{false, true} {
		for _, stream := range []bool{false, true} {
			t.Run(fmt.Sprintf("passthrough=%v/stream=%v", passthrough, stream), func(t *testing.T) {
				captured := make(chan subagentCapturedRequest, 16)
				upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					var body map[string]any
					if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
						http.Error(w, err.Error(), 400)
						return
					}
					// The upstream accepts string values in client_metadata, even when
					// an embedded Turn Metadata JSON string contains numeric fields.
					clientMetadata, _ := body["client_metadata"].(map[string]any)
					for key, value := range clientMetadata {
						if _, ok := value.(string); !ok {
							http.Error(w, fmt.Sprintf("Invalid type for 'client_metadata.%s': expected a string", key), http.StatusBadRequest)
							return
						}
					}
					state := "mock-state-" + r.Header.Get("Thread-Id")
					captured <- subagentCapturedRequest{r.Header.Clone(), body, state}
					w.Header().Set("Content-Type", "text/event-stream")
					w.Header().Set("X-Codex-Turn-State", state)
					fmt.Fprint(w, "data: {\"type\":\"response.output_text.delta\",\"delta\":\"mock answer\"}\n\n")
					fmt.Fprint(w, "data: {\"type\":\"response.completed\",\"response\":{\"id\":\"resp_mock\",\"status\":\"completed\",\"output\":[{\"type\":\"message\",\"role\":\"assistant\",\"content\":[{\"type\":\"output_text\",\"text\":\"mock answer\"}]}],\"usage\":{\"input_tokens\":11,\"output_tokens\":2}}}\n\ndata: [DONE]\n\n")
				}))
				defer upstream.Close()
				target, err := url.Parse(upstream.URL)
				require.NoError(t, err)
				account := newTestOAuthAccount(86, map[string]any{codexFingerprintModeExtraKey: "subagent", "openai_passthrough": passthrough})
				account.Credentials = map[string]any{"access_token": "mock-oauth-token", "chatgpt_account_id": "mock-account"}
				svc := &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &subagentHTTPTransport{target}}
				router := gin.New()
				router.POST("/v1/responses", func(c *gin.Context) {
					body, err := io.ReadAll(c.Request.Body)
					if err != nil {
						c.AbortWithStatus(400)
						return
					}
					key, _ := strconv.ParseInt(c.GetHeader("X-Test-Key"), 10, 64)
					c.Set("api_key", &APIKey{ID: key})
					SetOpenAIClientTransport(c, OpenAIClientTransportHTTP)
					if _, err := svc.Forward(c.Request.Context(), c, account, body); err != nil {
						t.Errorf("Forward failed: %v", err)
					}
				})
				gateway := httptest.NewServer(router)
				defer gateway.Close()
				send := func(thread, turn, state, input string, key int64) subagentCapturedRequest {
					body := map[string]any{
						"model": "gpt-5.6-luna", "stream": stream, "store": false,
						"instructions": "mock instructions", "prompt_cache_key": "mock-root-session",
						"input":           []any{map[string]any{"type": "message", "role": "user", "content": input}},
						"client_metadata": map[string]any{"session_id": "mock-root-session", "thread_id": thread, "turn_id": turn, "other": "kept"},
					}
					if strings.Contains(input, "continued") {
						body["tools"] = []any{map[string]any{
							"type": "function", "name": "mock_tool",
							"parameters": map[string]any{"type": "object", "properties": map[string]any{}},
						}}
						inputItems, _ := body["input"].([]any)
						body["input"] = append(inputItems,
							map[string]any{"type": "function_call", "call_id": "fc_mock", "name": "mock_tool", "arguments": "{}"},
							map[string]any{"type": "function_call_output", "call_id": "fc_mock", "output": "A-tool-result"},
						)
					}
					payload, err := json.Marshal(body)
					require.NoError(t, err)
					req, err := http.NewRequest(http.MethodPost, gateway.URL+"/v1/responses", bytes.NewReader(payload))
					require.NoError(t, err)
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("User-Agent", "Codex Desktop/0.153.4 (Windows 10.0.26200; x86_64) unknown")
					req.Header.Set("Session-Id", "mock-root-session")
					req.Header.Set("Thread-Id", thread)
					req.Header.Set("X-Test-Key", strconv.FormatInt(key, 10))
					req.Header.Set("X-Codex-Turn-Metadata", `{"thread_source":"user","parent_thread_id":"old-parent","sandbox":"mock","turn_started_at_unix_ms":1789183114387}`)
					if state != "" {
						req.Header.Set("X-Codex-Turn-State", state)
					}
					resp, err := gateway.Client().Do(req)
					require.NoError(t, err)
					responseBody, err := io.ReadAll(resp.Body)
					_ = resp.Body.Close()
					require.NoError(t, err)
					require.Equal(t, 200, resp.StatusCode, string(responseBody))
					require.Contains(t, string(responseBody), "mock answer")
					var got subagentCapturedRequest
					select {
					case got = <-captured:
					default:
						t.Fatal("upstream received no request")
					}
					assert.Equal(t, body["input"], got.body["input"], "no parent or other conversation input injected")
					assert.Equal(t, body["instructions"], got.body["instructions"])
					if body["tools"] != nil {
						assert.Equal(t, body["tools"], got.body["tools"])
					}
					assert.Equal(t, "mock-root-session", got.body["prompt_cache_key"])
					assert.Equal(t, "Bearer mock-oauth-token", got.headers.Get("Authorization"))
					assert.Equal(t, "collab_spawn", got.headers.Get("X-OpenAI-Subagent"))
					assert.Equal(t, "model=gpt-5.6-luna", got.headers.Get("X-Codex-Routing-Hint"))
					cm, _ := got.body["client_metadata"].(map[string]any)
					var meta map[string]any
					require.NoError(t, json.Unmarshal([]byte(got.headers.Get("X-Codex-Turn-Metadata")), &meta))
					assert.Equal(t, cm["x-codex-turn-metadata"], got.headers.Get("X-Codex-Turn-Metadata"))
					assert.Equal(t, got.headers.Get("Session-Id"), meta["parent_thread_id"])
					assert.Equal(t, got.headers.Get("Thread-Id"), meta["thread_id"])
					assert.Equal(t, got.headers.Get("Thread-Id"), got.headers.Get("X-Client-Request-Id"))
					assert.Equal(t, got.headers.Get("Thread-Id")+":0", meta["window_id"])
					assert.Equal(t, "0", cm["window_number"])
					assert.Equal(t, float64(0), meta["window_number"])
					assert.Equal(t, "subagent", meta["thread_source"])
					assert.Equal(t, "thread_spawn", meta["subagent_kind"])
					assert.Equal(t, "mock", meta["sandbox"])
					assert.Equal(t, "kept", cm["other"])
					return got
				}
				a := send("main", "turn-a", "", "A-only", 1)
				continued := send("main", "turn-a", a.state, "A-only continued", 1)
				assert.Equal(t, a.headers.Get("Thread-Id"), continued.headers.Get("Thread-Id"))
				assert.Equal(t, a.state, continued.headers.Get("X-Codex-Turn-State"))
				b := send("other-thread", "turn-b", a.state, "B-only", 1)
				assert.NotEqual(t, a.headers.Get("Thread-Id"), b.headers.Get("Thread-Id"))
				assert.Equal(t, a.headers.Get("Session-Id"), b.headers.Get("Session-Id"))
				assert.Empty(t, b.headers.Get("X-Codex-Turn-State"))
				native := send("native-child", "turn-c", "", "child-only", 1)
				assert.NotEqual(t, a.headers.Get("Thread-Id"), native.headers.Get("Thread-Id"))
				crossed := send("main", "turn-a", b.state, "A-only next", 1)
				assert.Empty(t, crossed.headers.Get("X-Codex-Turn-State"))
				otherUser := send("main", "turn-a", a.state, "other-user-only", 2)
				assert.NotEqual(t, a.headers.Get("Thread-Id"), otherUser.headers.Get("Thread-Id"))
				assert.Empty(t, otherUser.headers.Get("X-Codex-Turn-State"))
				// A fresh service models restart: deterministic IDs survive, unknown state does not.
				svc = &OpenAIGatewayService{cfg: &config.Config{}, httpUpstream: &subagentHTTPTransport{target}}
				restarted := send("main", "turn-a", a.state, "A after restart", 1)
				assert.Equal(t, a.headers.Get("Thread-Id"), restarted.headers.Get("Thread-Id"))
				assert.Empty(t, restarted.headers.Get("X-Codex-Turn-State"))
				assert.False(t, strings.Contains(restarted.headers.Get("X-Codex-Turn-Metadata"), "old-parent"))
			})
		}
	}
}
