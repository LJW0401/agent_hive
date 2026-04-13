package store

// LayoutEntry represents a container's position in the grid.
type LayoutEntry struct {
	ContainerID string `json:"containerId"`
	Page        int    `json:"page"`
	Position    int    `json:"position"`
}

// GetLayout returns all layout entries ordered by page and position.
func (s *Store) GetLayout() ([]LayoutEntry, error) {
	rows, err := s.db.Query(
		`SELECT container_id, page, position FROM layouts ORDER BY page ASC, position ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []LayoutEntry
	for rows.Next() {
		var e LayoutEntry
		if err := rows.Scan(&e.ContainerID, &e.Page, &e.Position); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SetLayout replaces the entire layout atomically.
func (s *Store) SetLayout(entries []LayoutEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM layouts`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO layouts (container_id, page, position) VALUES (?, ?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.ContainerID, e.Page, e.Position); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AddLayoutEntry adds a single container to the layout at the next available slot.
func (s *Store) AddLayoutEntry(containerID string, page, position int) error {
	_, err := s.db.Exec(
		`INSERT INTO layouts (container_id, page, position) VALUES (?, ?, ?)`,
		containerID, page, position,
	)
	return err
}

// RemoveLayoutEntry removes a container from the layout.
func (s *Store) RemoveLayoutEntry(containerID string) error {
	_, err := s.db.Exec(`DELETE FROM layouts WHERE container_id = ?`, containerID)
	return err
}

// NextAvailableSlot finds the next available page/position for a new container.
// Returns (page, position). Grid is 4 slots per page (0-3).
func (s *Store) NextAvailableSlot() (int, int, error) {
	entries, err := s.GetLayout()
	if err != nil {
		return 0, 0, err
	}

	// Build a set of occupied slots
	occupied := make(map[[2]int]bool)
	for _, e := range entries {
		occupied[[2]int{e.Page, e.Position}] = true
	}

	// Find the first empty slot
	for page := 0; page < 100; page++ {
		for pos := 0; pos < 4; pos++ {
			if !occupied[[2]int{page, pos}] {
				return page, pos, nil
			}
		}
	}

	return 0, 0, nil
}
