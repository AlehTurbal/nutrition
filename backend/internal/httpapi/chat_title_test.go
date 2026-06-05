package httpapi

import (
	"testing"
	"unicode/utf8"
)

func TestTitleFromMessage(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"short unchanged", "добавь творог", "добавь творог"},
		{"whitespace collapsed", "  добавь   творог  5% ", "добавь творог 5%"},
		{"newlines collapsed", "добавь\nтворог\tжирный", "добавь творог жирный"},
		{
			"long truncated on word boundary",
			"добавь продукт творог пять процентов и посчитай его калорийность пожалуйста",
			"добавь продукт творог пять процентов и посчитай…",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := titleFromMessage(c.in)
			if got != c.want {
				t.Errorf("titleFromMessage(%q) = %q, want %q", c.in, got, c.want)
			}
			if !utf8.ValidString(got) {
				t.Errorf("titleFromMessage(%q) = %q is not valid UTF-8", c.in, got)
			}
		})
	}
}
