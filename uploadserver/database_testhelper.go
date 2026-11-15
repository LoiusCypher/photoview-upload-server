package uploadserver

import (
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const SqliteDbInMemory = ":memory:"

// CreateDatabase_TestHelper for testing.
func CreateEmptyDatabase_TestHelper(t *testing.T, sqlPath string) {
	t.Helper()

	// _ = env.Load("../.env-test")

	// =========================================Setup Database
	dbPath := *SetDbPath(sqlPath)
	require.Equal( t, dbPath, sqlPath)
	if dbPath != SqliteDbInMemory {
		require.FileExists( t, dbPath)
	}
	db := openDb()
	require.NotNil( t, db)
	defer db.Close()
	tx, err3 := db.Begin()
	require.Nil( t, err3)
	_, err4 := tx.Exec("DELETE FROM shafiles")
	require.Nil( t, err4)
	defer tx.Rollback()
	err5 := tx.Commit()
	require.Nil( t, err5)
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

func RecordPdf_TestHelper(t *testing.T, pdfFilePath string, record_db_path string) string {
	shaCalc, err1 := calcSha( pdfFilePath)
	assert.Nil( t, err1)
	shaRec, err2 := recordSha( pdfFilePath, record_db_path)
	assert.Nil( t, err2)
	require.Equal( t, shaCalc, shaRec)
	return shaRec
}

func RecordFile_TestHelper(t *testing.T, tmpFilePath string, dir_db_path string, record_db_path string, db_entries_expected int, dir_entries_expected int) string {
	shaRec := RecordPdf_TestHelper( t, tmpFilePath, record_db_path)
	shaGet, err3 := getPathSha( record_db_path)
	assert.Nil( t, err3)
	assert.Equal( t, shaRec, shaGet)
	listInfos, err4 := listShas( "")
	assert.Nil( t, err4)
	assert.Equal( t, db_entries_expected, len(listInfos))
	dirInfos, err5 := listShas( dir_db_path)
	assert.Nil( t, err5)
	assert.Equal( t, dir_entries_expected, len(dirInfos))
	assert.Equal( t, shaRec, dirInfos[0].Sha)
	assert.Equal( t, record_db_path, dirInfos[0].Path)
	return shaRec
}

func DeleteFile_TestHelper(t *testing.T, dir_db_path string, record_db_path string, sha string, db_entries_expected int, dir_entries_expected int)  {
	shaDel, err1 := deleteSha( record_db_path)
	assert.Nil( t, err1)
	assert.Equal( t, sha, shaDel)
	listInfos, err2 := listShas( "")
	assert.Nil( t, err2)
	assert.Equal( t, db_entries_expected, len(listInfos))
	dirInfos, err3 := listShas( dir_db_path)
	assert.Nil( t, err3)
	assert.Equal( t, dir_entries_expected, len(dirInfos))
	shaGet, err4 := getPathSha( record_db_path)
	assert.Nil( t, err4)
	assert.Equal( t, "", shaGet)
}

