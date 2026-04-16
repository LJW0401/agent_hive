package store

import "testing"

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New() failed: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestCreateAndListTerminals(t *testing.T) {
	s := newTestStore(t)

	t1, err := s.CreateTerminal("c-1", "t-1", "Terminal 1", true)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if t1.SortOrder != 0 {
		t.Errorf("first terminal sort_order = %d, want 0", t1.SortOrder)
	}

	t2, err := s.CreateTerminal("c-1", "t-2", "Terminal 2", false)
	if err != nil {
		t.Fatalf("CreateTerminal: %v", err)
	}
	if t2.SortOrder != 1 {
		t.Errorf("second terminal sort_order = %d, want 1", t2.SortOrder)
	}

	list, err := s.ListTerminals("c-1")
	if err != nil {
		t.Fatalf("ListTerminals: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("ListTerminals len = %d, want 2", len(list))
	}
	if list[0].ID != "t-1" || list[1].ID != "t-2" {
		t.Errorf("terminals not in sort order: %v, %v", list[0].ID, list[1].ID)
	}
	if !list[0].IsDefault || list[1].IsDefault {
		t.Errorf("IsDefault flags wrong")
	}
}

func TestDeleteTerminal(t *testing.T) {
	s := newTestStore(t)

	s.CreateTerminal("c-1", "t-1", "T1", true)
	s.CreateTerminal("c-1", "t-2", "T2", false)

	if err := s.DeleteTerminal("t-2"); err != nil {
		t.Fatalf("DeleteTerminal: %v", err)
	}

	list, _ := s.ListTerminals("c-1")
	if len(list) != 1 {
		t.Errorf("after delete, len = %d, want 1", len(list))
	}
}

func TestDeleteNonExistentTerminal(t *testing.T) {
	s := newTestStore(t)
	// Should not error
	if err := s.DeleteTerminal("nonexistent"); err != nil {
		t.Errorf("DeleteTerminal(nonexistent) should not error: %v", err)
	}
}

func TestCountTerminals(t *testing.T) {
	s := newTestStore(t)

	count, err := s.CountTerminals("c-1")
	if err != nil {
		t.Fatalf("CountTerminals: %v", err)
	}
	if count != 0 {
		t.Errorf("empty count = %d, want 0", count)
	}

	s.CreateTerminal("c-1", "t-1", "T1", true)
	s.CreateTerminal("c-1", "t-2", "T2", false)

	count, err = s.CountTerminals("c-1")
	if err != nil {
		t.Fatalf("CountTerminals: %v", err)
	}
	if count != 2 {
		t.Errorf("count = %d, want 2", count)
	}
}

func TestListTerminalsEmpty(t *testing.T) {
	s := newTestStore(t)

	list, err := s.ListTerminals("c-nonexistent")
	if err != nil {
		t.Fatalf("ListTerminals: %v", err)
	}
	if list != nil && len(list) != 0 {
		t.Errorf("empty list should be nil or empty, got %d items", len(list))
	}
}

func TestGetTerminal(t *testing.T) {
	s := newTestStore(t)
	s.CreateTerminal("c-1", "t-1", "Terminal 1", true)

	tm, err := s.GetTerminal("t-1")
	if err != nil {
		t.Fatalf("GetTerminal: %v", err)
	}
	if tm.Name != "Terminal 1" || !tm.IsDefault {
		t.Errorf("GetTerminal returned wrong data: %+v", tm)
	}
}

func TestGetDefaultTerminal(t *testing.T) {
	s := newTestStore(t)
	s.CreateTerminal("c-1", "t-1", "Terminal 1", true)
	s.CreateTerminal("c-1", "t-2", "Terminal 2", false)

	dt, err := s.GetDefaultTerminal("c-1")
	if err != nil {
		t.Fatalf("GetDefaultTerminal: %v", err)
	}
	if dt.ID != "t-1" || !dt.IsDefault {
		t.Errorf("GetDefaultTerminal returned wrong terminal: %+v", dt)
	}
}
