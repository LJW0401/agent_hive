package store

import "time"

// Todo represents a todo item.
type Todo struct {
	ID        int64     `json:"id"`
	Container string    `json:"container"`
	Content   string    `json:"content"`
	Done      bool      `json:"done"`
	SortOrder int       `json:"sortOrder"`
	CreatedAt time.Time `json:"createdAt"`
}

// ListTodos returns all todos for a container, ordered by sort_order.
func (s *Store) ListTodos(containerID string) ([]Todo, error) {
	rows, err := s.db.Query(
		`SELECT id, container, content, done, sort_order, created_at
		 FROM todos WHERE container = ? ORDER BY sort_order ASC`,
		containerID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []Todo
	for rows.Next() {
		var t Todo
		var done int
		if err := rows.Scan(&t.ID, &t.Container, &t.Content, &done, &t.SortOrder, &t.CreatedAt); err != nil {
			return nil, err
		}
		t.Done = done != 0
		todos = append(todos, t)
	}
	return todos, rows.Err()
}

// CreateTodo creates a new todo. SortOrder is set to min-1 within the container,
// so the new item appears at the top of the ASC-ordered list.
func (s *Store) CreateTodo(containerID, content string) (*Todo, error) {
	var minOrder int
	_ = s.db.QueryRow(
		`SELECT COALESCE(MIN(sort_order), 1) FROM todos WHERE container = ?`,
		containerID,
	).Scan(&minOrder)

	newOrder := minOrder - 1

	result, err := s.db.Exec(
		`INSERT INTO todos (container, content, sort_order) VALUES (?, ?, ?)`,
		containerID, content, newOrder,
	)
	if err != nil {
		return nil, err
	}

	id, _ := result.LastInsertId()
	return &Todo{
		ID:        id,
		Container: containerID,
		Content:   content,
		Done:      false,
		SortOrder: newOrder,
		CreatedAt: time.Now(),
	}, nil
}

// UpdateTodo updates a todo's content and/or done status.
func (s *Store) UpdateTodo(id int64, content string, done bool) error {
	doneInt := 0
	if done {
		doneInt = 1
	}
	_, err := s.db.Exec(
		`UPDATE todos SET content = ?, done = ? WHERE id = ?`,
		content, doneInt, id,
	)
	return err
}

// DeleteTodo deletes a todo by ID.
func (s *Store) DeleteTodo(id int64) error {
	_, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	return err
}

// ReorderTodos updates the sort_order for a list of todo IDs.
// ids should be in the desired order.
func (s *Store) ReorderTodos(ids []int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`UPDATE todos SET sort_order = ? WHERE id = ?`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for i, id := range ids {
		if _, err := stmt.Exec(i, id); err != nil {
			return err
		}
	}

	return tx.Commit()
}

// DeleteTodosByContainer removes all todos for a container.
func (s *Store) DeleteTodosByContainer(containerID string) error {
	_, err := s.db.Exec(`DELETE FROM todos WHERE container = ?`, containerID)
	return err
}
