package stores

import (
	"context"
	"strings"
	"testing"
)

type fakeCompleter struct {
	gotSystem, gotUser string
	reply              string
	err                error
}

func (f *fakeCompleter) Complete(_ context.Context, system, user string) (string, error) {
	f.gotSystem, f.gotUser = system, user
	return f.reply, f.err
}

func TestMatchBuildsPromptAndParses(t *testing.T) {
	fc := &fakeCompleter{reply: `Here:
[{"needed":"Курица","matched":"Chicken breast 1kg","found":true},
 {"needed":"Соль","matched":"","found":false}]`}
	svc := NewService(fc)

	needed := []Needed{{Name: "Курица", Grams: 300}, {Name: "Соль", Grams: 5}}
	got, err := svc.Match(context.Background(), needed, "Chicken breast 1kg - 5.99\nMilk 1L - 1.20")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}

	if !strings.Contains(fc.gotUser, "Курица") || !strings.Contains(fc.gotUser, "Chicken breast 1kg") {
		t.Errorf("user prompt missing inputs: %q", fc.gotUser)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 results, got %d", len(got))
	}
	if got[0].Needed != "Курица" || got[0].Matched != "Chicken breast 1kg" || !got[0].Found {
		t.Errorf("result[0] wrong: %+v", got[0])
	}
	if got[1].Found {
		t.Errorf("result[1] should be not found: %+v", got[1])
	}
}

func TestMatchEmptyNeeded(t *testing.T) {
	svc := NewService(&fakeCompleter{})
	got, err := svc.Match(context.Background(), nil, "anything")
	if err != nil {
		t.Fatalf("Match: %v", err)
	}
	if len(got) != 0 {
		t.Errorf("expected empty result for no needed items, got %d", len(got))
	}
}
