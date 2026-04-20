package store

import (
	"testing"
)

// Smoke: new todos are inserted at the top of the ASC-ordered list.
func TestCreateTodoInsertsAtTop(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	first, err := s.CreateTodo("c-1", "first")
	if err != nil {
		t.Fatalf("CreateTodo first: %v", err)
	}
	second, err := s.CreateTodo("c-1", "second")
	if err != nil {
		t.Fatalf("CreateTodo second: %v", err)
	}
	third, err := s.CreateTodo("c-1", "third")
	if err != nil {
		t.Fatalf("CreateTodo third: %v", err)
	}

	if !(third.SortOrder < second.SortOrder && second.SortOrder < first.SortOrder) {
		t.Fatalf("expected sort_order strictly decreasing: first=%d second=%d third=%d",
			first.SortOrder, second.SortOrder, third.SortOrder)
	}

	todos, err := s.ListTodos("c-1")
	if err != nil {
		t.Fatalf("ListTodos: %v", err)
	}
	if len(todos) != 3 {
		t.Fatalf("expected 3 todos, got %d", len(todos))
	}
	if todos[0].Content != "third" || todos[1].Content != "second" || todos[2].Content != "first" {
		t.Fatalf("expected [third, second, first], got [%s, %s, %s]",
			todos[0].Content, todos[1].Content, todos[2].Content)
	}
}

// Edge: container isolation — min-1 is computed per-container.
func TestCreateTodoSortOrderPerContainer(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	// c-1 gets some todos first (sort_order: 0, -1, -2)
	_, _ = s.CreateTodo("c-1", "a")
	_, _ = s.CreateTodo("c-1", "b")
	_, _ = s.CreateTodo("c-1", "c")

	// c-2's first todo must not be influenced by c-1's values
	first, err := s.CreateTodo("c-2", "x")
	if err != nil {
		t.Fatalf("CreateTodo c-2: %v", err)
	}
	if first.SortOrder != 0 {
		t.Fatalf("c-2 first todo sort_order = %d, want 0", first.SortOrder)
	}
}

// Edge: after reorder normalises values to [0..N-1], a newly created todo
// still appears at the top (sort_order = -1 < 0).
func TestCreateTodoAfterReorderStillAtTop(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	a, _ := s.CreateTodo("c-1", "a")
	b, _ := s.CreateTodo("c-1", "b")

	// Reorder rewrites sort_order to 0, 1
	if err := s.ReorderTodos([]int64{a.ID, b.ID}); err != nil {
		t.Fatalf("ReorderTodos: %v", err)
	}

	fresh, err := s.CreateTodo("c-1", "fresh")
	if err != nil {
		t.Fatalf("CreateTodo fresh: %v", err)
	}
	if fresh.SortOrder >= 0 {
		t.Fatalf("fresh sort_order = %d, want < 0", fresh.SortOrder)
	}

	todos, _ := s.ListTodos("c-1")
	if todos[0].Content != "fresh" {
		t.Fatalf("expected fresh at top, got %q", todos[0].Content)
	}
}

// Edge: empty container — first todo gets sort_order 0 (min of empty set = 1, 1-1=0).
func TestCreateTodoFirstItemSortOrderZero(t *testing.T) {
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	defer s.Close()

	first, err := s.CreateTodo("c-empty", "only")
	if err != nil {
		t.Fatalf("CreateTodo: %v", err)
	}
	if first.SortOrder != 0 {
		t.Fatalf("first sort_order = %d, want 0", first.SortOrder)
	}
}
