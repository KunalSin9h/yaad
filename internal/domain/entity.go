package domain

import "time"

// EntityType classifies the kind of named entity extracted from a memory.
type EntityType string

const (
	EntityTypePerson  EntityType = "person"
	EntityTypePlace   EntityType = "place"
	EntityTypeProject EntityType = "project"
	EntityTypeConcept EntityType = "concept"
	EntityTypeTool    EntityType = "tool"
)

// Entity is a named thing extracted from memory content — a person, project,
// tool, concept, or place. Entities form the nodes of the knowledge graph;
// memory_entities links them to specific memories.
type Entity struct {
	ID        string
	Name      string
	Type      EntityType
	CreatedAt time.Time
}
