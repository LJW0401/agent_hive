package store

import "time"

// ContainerMeta is the persisted metadata for a container.
type ContainerMeta struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"createdAt"`
}

// SaveContainer persists container metadata.
func (s *Store) SaveContainer(id, name string) error {
	_, err := s.db.Exec(
		`INSERT OR REPLACE INTO containers (id, name) VALUES (?, ?)`,
		id, name,
	)
	return err
}

// DeleteContainerMeta removes container metadata.
func (s *Store) DeleteContainerMeta(id string) error {
	_, err := s.db.Exec(`DELETE FROM containers WHERE id = ?`, id)
	return err
}

// ListContainerMeta returns all saved containers.
func (s *Store) ListContainerMeta() ([]ContainerMeta, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM containers`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var metas []ContainerMeta
	for rows.Next() {
		var m ContainerMeta
		if err := rows.Scan(&m.ID, &m.Name, &m.CreatedAt); err != nil {
			return nil, err
		}
		metas = append(metas, m)
	}
	return metas, rows.Err()
}

// RenameContainer updates the name in the database.
func (s *Store) RenameContainer(id, name string) error {
	_, err := s.db.Exec(`UPDATE containers SET name = ? WHERE id = ?`, name, id)
	return err
}
