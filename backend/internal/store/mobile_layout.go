package store

// MobileLayoutEntry represents a container's position in the mobile layout.
type MobileLayoutEntry struct {
	ContainerID string `json:"containerId"`
	SortOrder   int    `json:"sortOrder"`
}

// GetMobileLayout returns all mobile layout entries ordered by sort_order.
func (s *Store) GetMobileLayout() ([]MobileLayoutEntry, error) {
	rows, err := s.db.Query(
		`SELECT container_id, sort_order FROM mobile_layouts ORDER BY sort_order ASC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []MobileLayoutEntry
	for rows.Next() {
		var e MobileLayoutEntry
		if err := rows.Scan(&e.ContainerID, &e.SortOrder); err != nil {
			return nil, err
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// SetMobileLayout replaces the entire mobile layout atomically.
func (s *Store) SetMobileLayout(entries []MobileLayoutEntry) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM mobile_layouts`); err != nil {
		return err
	}

	stmt, err := tx.Prepare(`INSERT INTO mobile_layouts (container_id, sort_order) VALUES (?, ?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, e := range entries {
		if _, err := stmt.Exec(e.ContainerID, e.SortOrder); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// AddMobileLayoutEntry appends a container to the mobile layout at the end.
func (s *Store) AddMobileLayoutEntry(containerID string) error {
	_, err := s.db.Exec(
		`INSERT INTO mobile_layouts (container_id, sort_order)
		 VALUES (?, COALESCE((SELECT MAX(sort_order) FROM mobile_layouts), -1) + 1)`,
		containerID,
	)
	return err
}

// RemoveMobileLayoutEntry removes a container from the mobile layout.
func (s *Store) RemoveMobileLayoutEntry(containerID string) error {
	_, err := s.db.Exec(`DELETE FROM mobile_layouts WHERE container_id = ?`, containerID)
	return err
}
