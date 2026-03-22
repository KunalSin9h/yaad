package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
)

// Compile-time interface check.
var _ ports.StoragePort = (*Store)(nil)

// Store implements ports.StoragePort backed by SQLite.
type Store struct {
	db *sql.DB
}

func (s *Store) Save(ctx context.Context, m *domain.Memory) error {
	emb, err := encodeEmbedding(m.Embedding)
	if err != nil {
		return fmt.Errorf("encode embedding: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memories
			(id, content, for_label, working_dir, hostname, created_at, remind_at, reminded_at, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content     = excluded.content,
			for_label   = excluded.for_label,
			remind_at   = excluded.remind_at,
			reminded_at = excluded.reminded_at,
			embedding   = excluded.embedding
	`,
		m.ID, m.Content, m.ForLabel,
		m.WorkingDir, m.Hostname, m.CreatedAt, m.RemindAt, m.RemindedAt, emb,
	)
	return err
}

func (s *Store) GetByID(ctx context.Context, id string) (*domain.Memory, error) {
	// ULIDs are 26 chars. Accept a prefix (e.g. the 10-char short ID shown
	// by `yaad add`) and match the first row whose ID starts with it.
	col := "id = ?"
	arg := id
	if len(id) < 26 {
		col = "id LIKE ?"
		arg = id + "%"
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, content, for_label, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE `+col+` LIMIT 1`, arg)
	return scanRow(row)
}

func (s *Store) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Memory, error) {
	q := `
		SELECT id, content, for_label, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE 1=1`
	args := []any{}

	if filter.OnlyReminders {
		q += " AND remind_at IS NOT NULL AND reminded_at IS NULL"
	}

	q += " ORDER BY created_at DESC"
	if filter.Limit > 0 {
		q += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}

	rows, err := s.db.QueryContext(ctx, q, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Memory
	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) Delete(ctx context.Context, id string) error {
	col := "id = ?"
	arg := id
	if len(id) < 26 {
		col = "id LIKE ?"
		arg = id + "%"
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM memories WHERE "+col, arg)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) DeleteAll(ctx context.Context) (int64, error) {
	res, err := s.db.ExecContext(ctx, "DELETE FROM memories")
	if err != nil {
		return 0, err
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// FindSimilar loads all embeddings from the DB, computes cosine similarity
// in-process, and returns the top-k most similar memories.
// Fine for personal scale (< 100k entries). Swap the adapter for chromadb etc.
// when scale demands it.
func (s *Store) FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*domain.Memory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, for_label, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	type scored struct {
		m     *domain.Memory
		score float64
	}
	var results []scored

	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		if len(m.Embedding) == 0 {
			continue
		}
		results = append(results, scored{m, cosineSimilarity(embedding, m.Embedding)})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}
	out := make([]*domain.Memory, topK)
	for i := range out {
		out[i] = results[i].m
	}
	return out, nil
}

func (s *Store) PendingReminders(ctx context.Context, before time.Time) ([]*domain.Memory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, for_label, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories
		WHERE remind_at IS NOT NULL AND reminded_at IS NULL AND remind_at <= ?`,
		before)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*domain.Memory
	for rows.Next() {
		m, err := scanRow(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) MarkReminded(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx,
		"UPDATE memories SET reminded_at = ? WHERE id = ?",
		time.Now(), id)
	return err
}

// --- scan helpers ---

// rowScanner is satisfied by both *sql.Row and *sql.Rows.
type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(s rowScanner) (*domain.Memory, error) {
	var m domain.Memory
	var embBlob []byte
	var remindAt, remindedAt sql.NullTime

	err := s.Scan(
		&m.ID, &m.Content, &m.ForLabel,
		&m.WorkingDir, &m.Hostname, &m.CreatedAt,
		&remindAt, &remindedAt, &embBlob,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if remindAt.Valid {
		m.RemindAt = &remindAt.Time
	}
	if remindedAt.Valid {
		m.RemindedAt = &remindedAt.Time
	}
	if len(embBlob) > 0 {
		if emb, err := decodeEmbedding(embBlob); err == nil {
			m.Embedding = emb
		}
	}
	return &m, nil
}

// --- embedding codec ---

func encodeEmbedding(v []float32) ([]byte, error) {
	if len(v) == 0 {
		return nil, nil
	}
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(v); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func decodeEmbedding(b []byte) ([]float32, error) {
	var v []float32
	if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&v); err != nil {
		return nil, err
	}
	return v, nil
}

// --- vector math ---

func cosineSimilarity(a, b []float32) float64 {
	if len(a) != len(b) || len(a) == 0 {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += float64(a[i]) * float64(b[i])
		normA += float64(a[i]) * float64(a[i])
		normB += float64(b[i]) * float64(b[i])
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}
