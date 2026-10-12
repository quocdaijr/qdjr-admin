package migrations

import (
	"math"
	"testing"

	"github.com/pressly/goose/v3"
)

// Versions must be unique and contiguous from 1: a duplicate or skipped
// number is a review mistake that goose would otherwise only surface against
// a real database.
func TestEmbeddedMigrationsAreContiguous(t *testing.T) {
	goose.SetBaseFS(FS)
	ms, err := goose.CollectMigrations(".", 0, math.MaxInt64)
	if err != nil {
		t.Fatal(err)
	}
	if len(ms) < 7 {
		t.Fatalf("expected at least 7 migrations, got %d", len(ms))
	}
	for i, m := range ms {
		if m.Version != int64(i+1) {
			t.Fatalf("%s has version %d, expected %d (numbering must be contiguous)", m.Source, m.Version, i+1)
		}
	}
}
