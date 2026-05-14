package dialog

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

type countingCompletionProvider struct {
	calls   int
	queries []string
}

func (p *countingCompletionProvider) GetId() string {
	return "test"
}

func (p *countingCompletionProvider) GetEntry() CompletionItemI {
	return NewCompletionItem(CompletionItem{
		Title: "Test",
		Value: "test",
	})
}

func (p *countingCompletionProvider) GetChildEntries(query string) ([]CompletionItemI, error) {
	p.calls++
	p.queries = append(p.queries, query)
	return []CompletionItemI{
		NewCompletionItem(CompletionItem{
			Title: "match",
			Value: "match",
		}),
	}, nil
}

func TestCompletionDialogDoesNotLoadEntriesDuringConstruction(t *testing.T) {
	provider := &countingCompletionProvider{}

	_ = NewCompletionDialogCmp(provider)

	if provider.calls != 0 {
		t.Fatalf("expected no completion lookups during construction, got %d", provider.calls)
	}
}

func TestCompletionDialogWaitsForNonEmptyQuery(t *testing.T) {
	provider := &countingCompletionProvider{}
	model := NewCompletionDialogCmp(provider)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'@'}})
	if provider.calls != 0 {
		t.Fatalf("expected no lookup for empty completion query, got %d", provider.calls)
	}

	_, _ = updated.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'m'}})
	if provider.calls != 1 {
		t.Fatalf("expected one lookup after a non-empty query, got %d", provider.calls)
	}
	if provider.queries[0] != "m" {
		t.Fatalf("expected query %q, got %q", "m", provider.queries[0])
	}
}
