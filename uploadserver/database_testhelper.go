package uploadserver

import (
	//"database/sql"
	"os"
	"path"
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateDatabase_TestHelper for testing.
func CreateDatabase_TestHelper(t *testing.T) {
	t.Helper()

	// _ = env.Load("../.env-test")

	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	// sqlPath := "/opt/app/shafiles.db"
	sqlMemo := ":memory:"
	sqlPath := "/tmp/shafiles.db"
	err1 := os.MkdirAll( path.Dir(sqlPath), 0755)
	assert.Nil( t, err1)
	dbPath := *SetDbPath(sqlPath)
	assert.Equal( t, dbPath, sqlPath)
	if dbPath != sqlMemo {
		require.FileExists( t, dbPath)
	}
	db := openDb()
	assert.NotNil( t, db)
	defer db.Close()
	tx, err3 := db.Begin()
	assert.Nil( t, err3)
	err4 := tx.Exec("DELETE FROM shafiles")
	assert.Nil( t, err4)
	err5 = tx.Commit()
	assert.Nil( t, err5)
	// =========================================Setup Database

	// if err := db.MigrateUp(context.Background()); err != nil {
		// t.Fatal(err)
	// }

	//return db
}

func CreateFile_TestHelper(t *testing.T, fs afero.Fs, tmpPath string, fileName string, content string) string {
	f, err2 := afero.TempFile( fs, tmpPath, fileName)
	assert.Nil( t, err2)
	cnt, err3 := f.WriteString( content)
	assert.Nil( t, err3)
	assert.Equal( t, cnt, len(content))
	tmpFilePath := f.Name()
	f.Close()
	return tmpFilePath
}

func RecordFile_TestHelper(t *testing.T, tmpFilePath string, dir_db_path string, record_db_path string, db_entries_expected int, dir_entries_expected int) string {
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
	assert.Equal( t, len(dirInfos), dir_entries_expected)
	assert.Equal( t, shaCalc, dirInfos[0].Sha)
	assert.Equal( t, record_db_path, dirInfos[0].Path)
	return shaCalc
}

func DeleteFile_TestHelper(t *testing.T, dir_db_path string, record_db_path string, sha string, db_entries_expected int, dir_entries_expected int)  {
	shaDel, err1 := deleteSha( record_db_path)
	assert.Nil( t, err1)
	assert.Equal( t, sha, shaDel)
	listInfos, err2 := listShas( "")
	assert.Nil( t, err2)
	assert.Equal( t, len(listInfos), db_entries_expected)
	dirInfos, err3 := listShas( dir_db_path)
	assert.Nil( t, err3)
	assert.Equal( t, len(dirInfos), dir_entries_expected)
	shaGet, err4 := getPathSha( record_db_path)
	assert.Nil( t, err4)
	assert.Equal( t, "", shaGet)
}

