package storage

import (
	"database/sql"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Message struct {
	ID        int64     `json:"id"`
	Author    string    `json:"author"`
	Channel   string    `json:"channel"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

type Store struct {
	db *sql.DB
}

func NewStore(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	s := &Store{db: db}
	if err := s.InitSchema(); err != nil {
		_ = db.Close()
		return nil, err
	}

	return s, nil
}

func (s *Store) InitSchema() error {
	schema := `
CREATE TABLE IF NOT EXISTS messages (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	author TEXT NOT NULL,
	channel TEXT NOT NULL,
	text TEXT NOT NULL,
	created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_messages_channel_created ON messages(channel, created_at);
`

	_, err := s.db.Exec(schema)
	return err
}

func (s *Store) SaveMessage(author, channel, text string) error {
	_, err := s.db.Exec(
		`INSERT INTO messages (author, channel, text, created_at) VALUES (?, ?, ?, ?)`,
		author,
		channel,
		text,
		time.Now().UTC().Format(time.RFC3339Nano),
	)
	return err
}

func (s *Store) RecentMessages(channel string, limit int) ([]Message, error) {
	if limit <= 0 {
		limit = 50
	}

	rows, err := s.db.Query(
		`SELECT id, author, channel, text, created_at FROM messages WHERE channel = ? ORDER BY id DESC LIMIT ?`,
		channel,
		limit,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	res := make([]Message, 0, limit)
	for rows.Next() {
		var m Message
		var createdStr string
		if err := rows.Scan(&m.ID, &m.Author, &m.Channel, &m.Text, &createdStr); err != nil {
			return nil, err
		}

		t, err := time.Parse(time.RFC3339Nano, createdStr)
		if err != nil {
			t = time.Now().UTC()
		}
		m.CreatedAt = t
		res = append(res, m)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Convert from DESC query order to chronological order for rendering.
	for i, j := 0, len(res)-1; i < j; i, j = i+1, j-1 {
		res[i], res[j] = res[j], res[i]
	}

	return res, nil
}
