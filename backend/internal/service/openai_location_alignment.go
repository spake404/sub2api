package service

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"github.com/tidwall/sjson"
)

const (
	alignedTargetCountry  = "US"
	alignedTargetRegion   = "Ohio"
	alignedTargetCity     = "Piketon"
	alignedTargetTimezone = "America/New_York"
)

var (
	envCurrentDateRegex = regexp.MustCompile(`<current_date>[^<]*</current_date>`)
	envTimezoneRegex    = regexp.MustCompile(`<timezone>[^<]*</timezone>`)
)

// alignOpenAILocationFields 对齐 OpenAI / Codex 请求中的环境地区与时区画像。
// 针对 codex 客户端上送的 <environment_context> 及 tools 中的 web_search，
// 抹除 Asia/Shanghai 等境内指纹特征，统一对齐为官方默认参考标准 America/New_York。
func alignOpenAILocationFields(body []byte) ([]byte, bool, error) {
	if len(body) == 0 {
		return body, false, nil
	}
	if !gjson.ValidBytes(body) {
		return body, false, nil
	}

	root := gjson.ParseBytes(body)
	if !root.IsObject() {
		return body, false, nil
	}

	changed := false
	normalized := body

	// 1. 检查并对齐 input 中的 <environment_context>
	input := root.Get("input")
	if input.IsArray() {
		loc, err := time.LoadLocation(alignedTargetTimezone)
		if err != nil {
			loc = time.FixedZone("EDT", -4*3600)
		}
		nyDate := time.Now().In(loc).Format("2006-01-02")
		newDateTag := fmt.Sprintf("<current_date>%s</current_date>", nyDate)
		newTzTag := fmt.Sprintf("<timezone>%s</timezone>", alignedTargetTimezone)

		inputIdx := 0
		input.ForEach(func(_, msg gjson.Result) bool {
			defer func() { inputIdx++ }()
			if msg.Get("role").String() != "user" {
				return true
			}

			// 检查 content
			content := msg.Get("content")
			if content.IsArray() {
				partIdx := 0
				content.ForEach(func(_, part gjson.Result) bool {
					defer func() { partIdx++ }()
					if part.Get("type").String() != "input_text" {
						return true
					}
					text := part.Get("text").String()
					if !strings.Contains(text, "<environment_context>") {
						return true
					}

					// 包含 <environment_context>，进行正则替换
					updatedText := text
					if envCurrentDateRegex.MatchString(updatedText) {
						updatedText = envCurrentDateRegex.ReplaceAllString(updatedText, newDateTag)
					}
					if envTimezoneRegex.MatchString(updatedText) {
						updatedText = envTimezoneRegex.ReplaceAllString(updatedText, newTzTag)
					}

					if updatedText != text {
						path := fmt.Sprintf("input.%d.content.%d.text", inputIdx, partIdx)
						if next, setErr := sjson.SetBytes(normalized, path, updatedText); setErr == nil {
							normalized = next
							changed = true
						}
					}
					return true
				})
			} else if content.Type == gjson.String {
				text := content.String()
				if strings.Contains(text, "<environment_context>") {
					updatedText := text
					if envCurrentDateRegex.MatchString(updatedText) {
						updatedText = envCurrentDateRegex.ReplaceAllString(updatedText, newDateTag)
					}
					if envTimezoneRegex.MatchString(updatedText) {
						updatedText = envTimezoneRegex.ReplaceAllString(updatedText, newTzTag)
					}
					if updatedText != text {
						path := fmt.Sprintf("input.%d.content", inputIdx)
						if next, setErr := sjson.SetBytes(normalized, path, updatedText); setErr == nil {
							normalized = next
							changed = true
						}
					}
				}
			}
			return true
		})
	}

	// 2. 检查并对齐 tools 中的 web_search
	tools := root.Get("tools")
	if tools.IsArray() {
		toolIdx := 0
		tools.ForEach(func(_, tool gjson.Result) bool {
			defer func() { toolIdx++ }()
			toolType := tool.Get("type").String()
			if toolType == "web_search" || strings.HasPrefix(toolType, "web_search_") {
				currentTz := tool.Get("user_location.timezone").String()
				if currentTz != alignedTargetTimezone {
					locMap := map[string]any{
						"type":     "approximate",
						"country":  alignedTargetCountry,
						"region":   alignedTargetRegion,
						"city":     alignedTargetCity,
						"timezone": alignedTargetTimezone,
					}
					path := fmt.Sprintf("tools.%d.user_location", toolIdx)
					if next, setErr := sjson.SetBytes(normalized, path, locMap); setErr == nil {
						normalized = next
						changed = true
					}
				}
			}
			return true
		})
	}

	return normalized, changed, nil
}
