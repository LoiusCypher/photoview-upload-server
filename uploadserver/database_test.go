package uploadserver

import (
	"os"
	"path"
	"path/filepath"
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// CreateDatabase_TestHelper for testing.
func CreateEmptyDatabase_TestHelper( t *testing.T, sqlPath string) {
	t.Helper()

	// _ = env.Load("../.env-test")

	// =========================================Setup Database
	dbPath := *SetDbPath(sqlPath)
	// require.Equal( t, dbPath, sqlPath)
	require.FileExists( t, dbPath)
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

func RecordFile_TestHelper( t *testing.T, pdfFilePath string, record_db_path string) string {
	shaCalc, err1 := calcSha( pdfFilePath)
	assert.Nil( t, err1)
	shaRec, err2 := recordFile( pdfFilePath, record_db_path)
	assert.Nil( t, err2)
	require.Equal( t, shaCalc, shaRec)
	return shaRec
}

func RecordPdfSha_TestHelper( t *testing.T, pdfFileSha string, record_db_path string) string {
	shaRec, err2 := recordFileSha( pdfFileSha, record_db_path)
	assert.Nil( t, err2)
	require.Equal( t, pdfFileSha, shaRec)
	return shaRec
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

func RecordFile_Test( t *testing.T, tmpFilePath string, dir_db_path string, record_db_path string, db_entries_expected int, dir_entries_expected int) string {
	shaRec := RecordFile_TestHelper( t, tmpFilePath, record_db_path)
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

func DeleteFile_Test( t *testing.T, dir_db_path string, record_db_path string, sha string, db_entries_expected int, dir_entries_expected int)  {
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

const empty_sha_std string = "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
const abcde_sha_std string = "248d6a61d20638b8e5c026930c3e6039a33ce45964ff2167f6ecedd419db06c1"

func TestDatabase(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	// =========================================Setup Database
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	//var fileInfos []FileInfo
	fileInfos, err := listShas( "")
	assert.Nil( t, err)
	assert.Equal( t, len(fileInfos), 0)
	// =========================================Add a file
	tmpPath := "/tmp"
	err = fs.MkdirAll( filepath.Dir(tmpPath), 0755)
	assert.Nil( t, err)
	// =========================================Add a file
	tmpFilePath_empty := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "")
	emptyFile_db_dir := "arbitrary / file "
	emptyFile_db_path := emptyFile_db_dir + "/" + " path"
	sha_empty := RecordFile_Test( t, tmpFilePath_empty, emptyFile_db_dir, emptyFile_db_path, 1, 1)
	assert.Equal( t, sha_empty, empty_sha_std)
	// =========================================Add another file
	tmpFilePath_hello := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello")
	helloFile_db_dir := "arbitrary / say "
	helloFile_db_path := helloFile_db_dir + "/" + " hello"
	sha_hello := RecordFile_Test( t, tmpFilePath_hello, helloFile_db_dir, helloFile_db_path, 2, 1)
	// =========================================Add a third file
	tmpFilePath_abcde := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq")
	abcdeFile_db_dir := "arbitrary / say / test"
	abcdeFile_db_path := abcdeFile_db_dir + "/" + " abcde "
	sha_abcde := RecordFile_Test( t, tmpFilePath_abcde, abcdeFile_db_dir, abcdeFile_db_path, 3, 1)
	assert.Equal( t, sha_abcde, abcde_sha_std)
	// =========================================Check that we can delete entries
	assert.NotEqual( t, sha_empty, sha_hello)
	DeleteFile_Test( t, emptyFile_db_dir, emptyFile_db_path, sha_empty, 2, 0)
	// ========================================Check tat we con overwrite
	tmpFilePath_hello2 := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello 2")
	sha_hello2 := RecordFile_Test( t, tmpFilePath_hello2, helloFile_db_dir, helloFile_db_path, 2, 2)
	assert.NotEqual( t, sha_hello, sha_hello2)
	// =========================================
}

