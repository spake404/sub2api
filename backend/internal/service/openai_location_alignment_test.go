package service

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
)

func TestAlignOpenAILocationFields(t *testing.T) {
	raw := `{
		"model": "gpt-6-astra",
		"input": [
			{
				"role": "user",
				"content": [
					{
						"type": "input_text",
						"text": "<environment_context>\n  <cwd>/Users/admin/project</cwd>\n  <current_date>2026-09-15</current_date>\n  <timezone>Asia/Shanghai</timezone>\n</environment_context>"
					}
				]
			}
		],
		"tools": [
			{
				"type": "web_search"
			}
		]
	}`

	aligned, changed, err := alignOpenAILocationFields([]byte(raw))
	require.NoError(t, err)
	require.True(t, changed)

	text := gjson.GetBytes(aligned, "input.0.content.0.text").String()
	require.Contains(t, text, "<timezone>America/New_York</timezone>")
	require.NotContains(t, text, "Asia/Shanghai")

	loc, _ := time.LoadLocation("America/New_York")
	if loc == nil {
		loc = time.FixedZone("EDT", -4*3600)
	}
	expectedDate := time.Now().In(loc).Format("2006-01-02")
	require.Contains(t, text, fmt.Sprintf("<current_date>%s</current_date>", expectedDate))

	toolLoc := gjson.GetBytes(aligned, "tools.0.user_location")
	require.True(t, toolLoc.Exists())
	require.Equal(t, "US", toolLoc.Get("country").String())
	require.Equal(t, "Ohio", toolLoc.Get("region").String())
	require.Equal(t, "Piketon", toolLoc.Get("city").String())
	require.Equal(t, "America/New_York", toolLoc.Get("timezone").String())
}
