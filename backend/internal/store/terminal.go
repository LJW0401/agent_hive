package store

import "time"

// TerminalMeta is the persisted metadata for a terminal within a container.
type TerminalMeta struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"containerId"`
	Name        string    `json:"name"`
	IsDefault   bool      `json:"isDefault"`
	SortOrder   int       `json:"sortOrder"`
	CreatedAt   time.Time `json:"createdAt"`
	LastCWD     string    `json:"lastCwd,omitempty"`
}

// CreateTerminal creates a new terminal for a container.
func (s *Store) CreateTerminal(containerID, id, name string, isDefault bool) (*TerminalMeta, error) {
	var maxOrder int
	_ = s.db.QueryRow(
		`SELECT COALESCE(MAX(sort_order), -1) FROM terminals WHERE container_id = ?`,
		containerID,
	).Scan(&maxOrder)

	now := time.Now()
	_, err := s.db.Exec(
		`INSERT INTO terminals (id, container_id, name, is_default, sort_order, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		id, containerID, name, boolToInt(isDefault), maxOrder+1, now,
	)
	if err != nil {
		return nil, err
	}

	return &TerminalMeta{
		ID:          id,
		ContainerID: containerID,
		Name:        name,
		IsDefault:   isDefault,
		SortOrder:   maxOrder + 1,
		CreatedAt:   now,
	}, nil
}

// ListTerminals returns all terminals for a container, ordered by sort_order.
func (s *Store) ListTerminals(containerID string) ([]TerminalMeta, error) {
	rows, err := s.db.Query(
		`SELECT id, container_id, name, is_default, sort_order, created_at, last_cwd
		 FROM terminals WHERE container_id = ? ORDER BY sort_order ASC`,
		containerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var terminals []TerminalMeta
	for rows.Next() {
		var t TerminalMeta
		var isDefault int
		if err := rows.Scan(&t.ID, &t.ContainerID, &t.Name, &isDefault, &t.SortOrder, &t.CreatedAt, &t.LastCWD); err != nil {
			return nil, err
		}
		t.IsDefault = isDefault != 0
		terminals = append(terminals, t)
	}
	return terminals, rows.Err()
}

// DeleteTerminal deletes a terminal by ID.
func (s *Store) DeleteTerminal(id string) error {
	_, err := s.db.Exec(`DELETE FROM terminals WHERE id = ?`, id)
	return err
}

// CountTerminals returns the number of terminals for a container.
func (s *Store) CountTerminals(containerID string) (int, error) {
	var count int
	err := s.db.QueryRow(
		`SELECT COUNT(*) FROM terminals WHERE container_id = ?`,
		containerID,
	).Scan(&count)
	return count, err
}

// DeleteTerminalsByContainer removes all terminals for a container.
func (s *Store) DeleteTerminalsByContainer(containerID string) error {
	_, err := s.db.Exec(`DELETE FROM terminals WHERE container_id = ?`, containerID)
	return err
}

// GetTerminal returns a single terminal by ID.
func (s *Store) GetTerminal(id string) (*TerminalMeta, error) {
	var t TerminalMeta
	var isDefault int
	err := s.db.QueryRow(
		`SELECT id, container_id, name, is_default, sort_order, created_at, last_cwd
		 FROM terminals WHERE id = ?`, id,
	).Scan(&t.ID, &t.ContainerID, &t.Name, &isDefault, &t.SortOrder, &t.CreatedAt, &t.LastCWD)
	if err != nil {
		return nil, err
	}
	t.IsDefault = isDefault != 0
	return &t, nil
}

// GetDefaultTerminal returns the default terminal for a container.
func (s *Store) GetDefaultTerminal(containerID string) (*TerminalMeta, error) {
	var t TerminalMeta
	var isDefault int
	err := s.db.QueryRow(
		`SELECT id, container_id, name, is_default, sort_order, created_at, last_cwd
		 FROM terminals WHERE container_id = ? AND is_default = 1`, containerID,
	).Scan(&t.ID, &t.ContainerID, &t.Name, &isDefault, &t.SortOrder, &t.CreatedAt, &t.LastCWD)
	if err != nil {
		return nil, err
	}
	t.IsDefault = isDefault != 0
	return &t, nil
}

// UpdateTerminalCWD records the last known working directory of a terminal's shell,
// so that reopening the terminal (after shell exit or server restart) can start in
// the same directory.
func (s *Store) UpdateTerminalCWD(id, cwd string) error {
	_, err := s.db.Exec(`UPDATE terminals SET last_cwd = ? WHERE id = ?`, cwd, id)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
