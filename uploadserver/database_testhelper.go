package uploadserver

import (
	// "context"
	"database/sql"
	"path"
	// "path/filepath"
	"testing"
	"github.com/spf13/afero"

	// "github.com/maragudk/env"
)

// CreateDatabase_TestHelper for testing.
//func CreateDatabase_TestHelper(t *testing.T) *sql.DB {
func CreateDatabase_TestHelper(t *testing.T) {
	t.Helper()

	// _ = env.Load("../.env-test")

	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	sqlPath := "/opt/app/shafiles.db"
	err1 := fs.MkdirAll( path.Dir(sqlPath), 0755)
	assert.Nil( t, err1)
	dbPath := *SetDbPath(sqlPath)
	assert.Equal( t, dbPath, sqlPath)
	found, err2 := afero.Exists(fs, sqlPath)
	assert.Nil( t, err2)
	db := openDb()
	assert.NotNil( t, db)
	defer db.Close()
	// =========================================Setup Database

	// if err := db.MigrateUp(context.Background()); err != nil {
		// t.Fatal(err)
	// }

	//return db
}

func CreateFile_TestHelper(t *testing.T, fs afero.FS, tmpPath string, fileName string, content string) string {
	f, err2 := afero.TempFile( fs, tmpPath, fileName)
	assert.Nil( t, err2)
	cnt, err3 := f.WriteString( content)
	assert.Nil( t, err3)
	assert.Equal( t, cnt, len(content))
	tmpFilePath := f.Name()
	f.Close()
	return tmpFilePath
}

func RecordFile_TestHelper(t *testing.T, tmpFilePath string, dir_db_path string, record_db_path string, db_entries_expected int) string {
	shaCalc, err1 := calcSha( tmpFilePath)
	assert.Nil( t, err1)
	shaRec, err2 := recordSha( tmpFilePath, record_db_path)
	assert.Nil( t, err2)
	assert.Equal( t, shaCalc, shaRec)
	shaGet, err3 := getPathSha( record_db_path)
	assert.Nil( t, err3)
	assert.Equal( t, shaCalc, shaGet)
	listInfos, err4 := listShas( "")
	assert.Nil( t, err4)
	assert.Equal( t, len(listInfos), db_entries_expected)
	dirInfos, err5 := listShas( dir_db_path)
	assert.Nil( t, err5)
	assert.Equal( t, len(dirInfos), 1)
	assert.Equal( t, shaCalc, dirInfos[0].Sha)
	assert.Equal( t, record_db_path, dirInfos[0].Path)
	return shaCalc
}

