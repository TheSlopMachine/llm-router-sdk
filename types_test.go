package sdk

import (
	"encoding/json"
	"testing"
)

func TestChatCompletionRequestUnmarshalStructuredContent(t *testing.T) {
	raw := []byte(`{
		"model":"kiro/claude-sonnet-4.5",
		"messages":[
			{"role":"user","content":[{"type":"text","text":"Plan this task"},{"type":"tool_result","tool_use_id":"call_1","content":[{"type":"text","text":"done"}]}]},
			{"role":"assistant","tool_calls":[{"id":"call_1","type":"function","function":{"name":"TodoWrite","arguments":"{\"items\":[\"a\"]}"}}],"content":""},
			{"role":"tool","tool_call_id":"call_1","content":"tool output"}
		],
		"tools":[
			{"type":"function","function":{"name":"TodoWrite","description":"write todos","parameters":{"type":"object"}}}
		]
	}`)

	var req ChatCompletionRequest
	if err := json.Unmarshal(raw, &req); err != nil {
		t.Fatalf("unmarshal request: %v", err)
	}

	if got := req.Messages[0].Content; got != "Plan this task\ndone" {
		t.Fatalf("expected flattened structured content, got %q", got)
	}
	if len(req.Messages[0].ContentParts) != 2 {
		t.Fatalf("expected structured content parts, got %d", len(req.Messages[0].ContentParts))
	}
	if req.Messages[1].ToolCalls[0].Function.Name != "TodoWrite" {
		t.Fatalf("expected tool call name TodoWrite, got %#v", req.Messages[1].ToolCalls)
	}
	if req.Messages[2].ToolCallID != "call_1" {
		t.Fatalf("expected tool_call_id call_1, got %q", req.Messages[2].ToolCallID)
	}
	if len(req.Tools) != 1 || req.Tools[0].Function == nil || req.Tools[0].Function.Name != "TodoWrite" {
		t.Fatalf("expected request tools to be preserved, got %#v", req.Tools)
	}
}

func TestChatMessageMarshalStructuredContent(t *testing.T) {
	msg := ChatMessage{
		Role: "user",
		ContentParts: []ChatMessageContentPart{
			{Type: "text", Text: "hello"},
			{Type: "text", Text: "world"},
		},
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal message: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("decode marshaled message: %v", err)
	}

	content, ok := decoded["content"].([]any)
	if !ok || len(content) != 2 {
		t.Fatalf("expected array content when content parts are present, got %#v", decoded["content"])
	}
}
