package uploadserver

import (
	// "context"
	"testing"
	"github.com/spf13/afero"
	_ "github.com/mattn/go-sqlite3"

	// "github.com/maragudk/env"
)

// CreateDatabase_TestHelper for testing.
func CreateDatabase_TestHelper(t *testing.T) *sql.DB {
	t.Helper()

	// _ = env.Load("../.env-test")

	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	sqlPath := "/opt/app/shafiles.db"
	if err := fs.MkdirAll( path.Dir(sqlPath), 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	dbPath := *SetDbPath(sqlPath)
	if dbPath != sqlPath {
		t.Fatalf("set db path failed %s", dbPath)
	}
	found, err := afero.Exists(fs, sqlPath)
	if err != nil {
		t.Fatalf("db file check error: %v", err)
	}
	if !found {
		t.Error("db file was not created")
	}
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db")
	}
	// =========================================Setup Database

	// if err := db.MigrateUp(context.Background()); err != nil {
		// t.Fatal(err)
	// }

	return db
}
