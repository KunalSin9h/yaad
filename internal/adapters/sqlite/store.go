package sqlite

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"math"
	"sort"
	"time"

	"github.com/kunalsin9h/yaad/internal/domain"
	"github.com/kunalsin9h/yaad/internal/ports"
	"github.com/oklog/ulid/v2"
)

// Compile-time interface check.
var _ ports.StoragePort = (*Store)(nil)

// Store implements ports.StoragePort backed by SQLite.
type Store struct {
	db *sql.DB
}

func (s *Store) Save(ctx context.Context, m *domain.Memory) error {
	tags, err := json.Marshal(m.Tags)
	if err != nil {
		return fmt.Errorf("marshal tags: %w", err)
	}
	emb, err := encodeEmbedding(m.Embedding)
	if err != nil {
		return fmt.Errorf("encode embedding: %w", err)
	}

	_, err = s.db.ExecContext(ctx, `
		INSERT INTO memories
			(id, content, for_label, type, tags, working_dir, hostname, created_at, remind_at, reminded_at, embedding)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			content     = excluded.content,
			for_label   = excluded.for_label,
			type        = excluded.type,
			tags        = excluded.tags,
			remind_at   = excluded.remind_at,
			reminded_at = excluded.reminded_at,
			embedding   = excluded.embedding
	`,
		m.ID, m.Content, m.ForLabel, string(m.Type), string(tags),
		m.WorkingDir, m.Hostname, m.CreatedAt, m.RemindAt, m.RemindedAt, emb,
	)
	return err
}

func (s *Store) GetByID(ctx context.Context, id string) (*domain.Memory, error) {
	col := "id = ?"
	arg := id
	if len(id) < 26 {
		col = "id LIKE ?"
		arg = id + "%"
	}
	row := s.db.QueryRowContext(ctx, `
		SELECT id, content, for_label, type, tags, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE `+col+` LIMIT 1`, arg)
	return scanRow(row)
}

func (s *Store) List(ctx context.Context, filter domain.ListFilter) ([]*domain.Memory, error) {
	q := `
		SELECT id, content, for_label, type, tags, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE 1=1`
	args := []any{}

	if filter.Type != "" {
		q += " AND type = ?"
		args = append(args, string(filter.Type))
	}
	if filter.Tag != "" {
		q += " AND tags LIKE ?"
		args = append(args, "%"+filter.Tag+"%")
	}
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
// Fine for personal scale (< 100k entries). Use FindHybrid for better quality.
func (s *Store) FindSimilar(ctx context.Context, embedding []float32, topK int) ([]*domain.Memory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, for_label, type, tags, working_dir, hostname,
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

// rrfK is the constant in the RRF formula. 60 is the standard value from
// the original RRF paper (Cormack et al., 2009). It smooths rank differences
// and prevents very high ranks from dominating.
const rrfK = 60

// FindHybrid performs hybrid retrieval by merging BM25 full-text search
// (SQLite FTS5) and cosine vector similarity using Reciprocal Rank Fusion.
//
// RRF score: score(d) = 1/(k + rank_bm25) + 1/(k + rank_vector)
//
// This outperforms vector-only search when the query contains exact keywords,
// names, commands, or tags, while matching or exceeding it on purely semantic
// queries. BM25 catches what vectors miss (exact terms); vectors catch what
// BM25 misses (semantic paraphrases).
func (s *Store) FindHybrid(ctx context.Context, query string, embedding []float32, topK int) ([]*domain.Memory, error) {
	// --- BM25 leg via FTS5 ---
	// Fetch a larger candidate pool (topK * 4) before RRF fusion.
	candidateN := topK * 4
	if candidateN < 20 {
		candidateN = 20
	}

	bm25Rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.content, m.for_label, m.type, m.tags,
		       m.working_dir, m.hostname, m.created_at, m.remind_at,
		       m.reminded_at, m.embedding
		FROM memories_fts f
		JOIN memories m ON m.rowid = f.rowid
		WHERE memories_fts MATCH ?
		ORDER BY bm25(memories_fts)
		LIMIT ?`, query, candidateN)
	if err != nil {
		// FTS query can fail on special characters — fall back to vector-only.
		return s.FindSimilar(ctx, embedding, topK)
	}
	defer bm25Rows.Close()

	bm25Rank := map[string]int{} // memory ID → 1-based rank
	var bm25IDs []string
	bm25Memories := map[string]*domain.Memory{}

	rank := 1
	for bm25Rows.Next() {
		m, err := scanRow(bm25Rows)
		if err != nil {
			return nil, err
		}
		bm25Rank[m.ID] = rank
		bm25IDs = append(bm25IDs, m.ID)
		bm25Memories[m.ID] = m
		rank++
	}
	if err := bm25Rows.Err(); err != nil {
		return nil, err
	}

	// --- Vector leg ---
	vecRows, err := s.db.QueryContext(ctx, `
		SELECT id, content, for_label, type, tags, working_dir, hostname,
		       created_at, remind_at, reminded_at, embedding
		FROM memories WHERE embedding IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer vecRows.Close()

	type vecScored struct {
		m     *domain.Memory
		score float64
	}
	var vecResults []vecScored

	for vecRows.Next() {
		m, err := scanRow(vecRows)
		if err != nil {
			return nil, err
		}
		if len(m.Embedding) == 0 {
			continue
		}
		vecResults = append(vecResults, vecScored{m, cosineSimilarity(embedding, m.Embedding)})
	}
	if err := vecRows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(vecResults, func(i, j int) bool {
		return vecResults[i].score > vecResults[j].score
	})

	vecRank := map[string]int{}
	vecMemories := map[string]*domain.Memory{}
	for i, r := range vecResults {
		vecRank[r.m.ID] = i + 1
		vecMemories[r.m.ID] = r.m
	}

	// --- RRF fusion ---
	// Collect all candidate IDs from both legs.
	allIDs := make(map[string]struct{})
	for id := range bm25Rank {
		allIDs[id] = struct{}{}
	}
	for id := range vecRank {
		allIDs[id] = struct{}{}
	}

	type rrfScored struct {
		id    string
		score float64
	}
	fused := make([]rrfScored, 0, len(allIDs))
	maxBM25Rank := len(bm25IDs) + 1
	maxVecRank := len(vecResults) + 1

	for id := range allIDs {
		bRank, hasBM25 := bm25Rank[id]
		vRank, hasVec := vecRank[id]

		if !hasBM25 {
			bRank = maxBM25Rank // penalise missing from BM25 leg
		}
		if !hasVec {
			vRank = maxVecRank // penalise missing from vector leg
		}

		score := 1.0/float64(rrfK+bRank) + 1.0/float64(rrfK+vRank)
		fused = append(fused, rrfScored{id, score})
	}

	sort.Slice(fused, func(i, j int) bool {
		return fused[i].score > fused[j].score
	})

	if topK > len(fused) {
		topK = len(fused)
	}

	out := make([]*domain.Memory, 0, topK)
	for _, f := range fused[:topK] {
		// Prefer the memory object from the vector leg (it always has embeddings).
		if m, ok := vecMemories[f.id]; ok {
			out = append(out, m)
		} else if m, ok := bm25Memories[f.id]; ok {
			out = append(out, m)
		}
	}
	return out, nil
}

// SaveEntities upserts the given entities and links them to memoryID.
// This is called after entity extraction during memory creation.
func (s *Store) SaveEntities(ctx context.Context, memoryID string, entities []domain.Entity) error {
	if len(entities) == 0 {
		return nil
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, e := range entities {
		// Upsert entity (name + type is unique).
		_, err := tx.ExecContext(ctx, `
			INSERT INTO entities (id, name, type, created_at)
			VALUES (?, ?, ?, ?)
			ON CONFLICT(name, type) DO NOTHING`,
			ulid.Make().String(), e.Name, string(e.Type), time.Now(),
		)
		if err != nil {
			return fmt.Errorf("upsert entity %q: %w", e.Name, err)
		}

		// Link entity to memory.
		_, err = tx.ExecContext(ctx, `
			INSERT INTO memory_entities (memory_id, entity_id)
			SELECT ?, id FROM entities WHERE name = ? AND type = ?
			ON CONFLICT DO NOTHING`,
			memoryID, e.Name, string(e.Type),
		)
		if err != nil {
			return fmt.Errorf("link entity %q: %w", e.Name, err)
		}
	}

	return tx.Commit()
}

// FindByEntities retrieves memories that reference any of the given entity names.
// Useful for "show me everything about Alice" or "memories related to payments-api".
func (s *Store) FindByEntities(ctx context.Context, names []string, topK int) ([]*domain.Memory, error) {
	if len(names) == 0 {
		return nil, nil
	}

	// Build placeholders for IN clause.
	placeholders := make([]byte, 0, len(names)*2)
	args := make([]any, 0, len(names)+1)
	for i, n := range names {
		if i > 0 {
			placeholders = append(placeholders, ',')
		}
		placeholders = append(placeholders, '?')
		args = append(args, n)
	}
	args = append(args, topK)

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT m.id, m.content, m.for_label, m.type, m.tags,
		       m.working_dir, m.hostname, m.created_at, m.remind_at,
		       m.reminded_at, m.embedding
		FROM memories m
		JOIN memory_entities me ON m.id = me.memory_id
		JOIN entities e ON me.entity_id = e.id
		WHERE e.name IN (`+string(placeholders)+`)
		ORDER BY m.created_at DESC
		LIMIT ?`, args...)
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

func (s *Store) PendingReminders(ctx context.Context, before time.Time) ([]*domain.Memory, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, content, for_label, type, tags, working_dir, hostname,
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanRow(s rowScanner) (*domain.Memory, error) {
	var m domain.Memory
	var tagsJSON, memType string
	var embBlob []byte
	var remindAt, remindedAt sql.NullTime

	err := s.Scan(
		&m.ID, &m.Content, &m.ForLabel, &memType, &tagsJSON,
		&m.WorkingDir, &m.Hostname, &m.CreatedAt,
		&remindAt, &remindedAt, &embBlob,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	m.Type = domain.MemoryType(memType)

	if err := json.Unmarshal([]byte(tagsJSON), &m.Tags); err != nil {
		m.Tags = []string{}
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
