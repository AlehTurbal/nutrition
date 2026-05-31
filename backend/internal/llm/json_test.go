package llm

import "testing"

func TestExtractJSON(t *testing.T) {
	cases := []struct {
		name, in, want string
	}{
		{"plain object", `{"a":1}`, `{"a":1}`},
		{"object in prose", "Sure! Here:\n{\"a\":1}\nDone.", `{"a":1}`},
		{"array in prose", "result: [1,2,3] ok", `[1,2,3]`},
		{"fenced", "```json\n{\"a\":1}\n```", `{"a":1}`},
		{"nested braces", `prefix {"a":{"b":2}} suffix`, `{"a":{"b":2}}`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := ExtractJSON(c.in)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != c.want {
				t.Errorf("ExtractJSON(%q) = %q, want %q", c.in, got, c.want)
			}
		})
	}
}

func TestExtractJSONNone(t *testing.T) {
	if _, err := ExtractJSON("no json here"); err == nil {
		t.Error("expected error when no JSON present")
	}
}
