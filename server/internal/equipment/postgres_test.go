package equipment

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/maillunex-lang/NewMMO/server/internal/quest"
)

// The published canon build must yield the full 5 classes × 6 regions × 9 slots matrix.
func TestCatalogFromPublishedBuild(t *testing.T) {
	if os.Getenv("NEWMMO_REQUIRE_ROLE_URLS") != "1" && os.Getenv("NEWMMO_TEST_APP_DATABASE_URL") == "" {
		t.Skip("set NEWMMO_TEST_APP_DATABASE_URL for equipment integration")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, os.Getenv("NEWMMO_TEST_APP_DATABASE_URL"))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	build, err := quest.NewCatalog().ActiveBuildID(ctx, pool)
	if err != nil {
		t.Skipf("no published content build: %v", err)
	}
	c := NewCatalog()
	if err := c.LoadFromDB(ctx, pool, build); err != nil {
		t.Fatal(err)
	}
	if c.Len() != 270 {
		t.Fatalf("templates = %d, want 270 (canon 11 §6)", c.Len())
	}
	for cls := range classShares {
		for r := 1; r <= 6; r++ {
			for _, s := range Slots {
				if len(c.Candidates(cls, "REG-0"+string(rune('0'+r)), s)) == 0 {
					t.Fatalf("no template for %s REG-0%d %s", cls, r, s)
				}
			}
		}
	}
}
