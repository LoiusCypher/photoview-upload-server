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
	sha_empty := RecordFile_TestHelper( t, tmpFilePath_empty, emptyFile_db_dir, emptyFile_db_path, 1, 1)
	assert.Equal( t, sha_empty, empty_sha_std)
	// =========================================Add another file
	tmpFilePath_hello := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello")
	helloFile_db_dir := "arbitrary / say "
	helloFile_db_path := helloFile_db_dir + "/" + " hello"
	sha_hello := RecordFile_TestHelper( t, tmpFilePath_hello, helloFile_db_dir, helloFile_db_path, 2, 1)
	// =========================================Add a third file
	tmpFilePath_abcde := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "abcdbcdecdefdefgefghfghighijhijkijkljklmklmnlmnomnopnopq")
	abcdeFile_db_dir := "arbitrary / say / test"
	abcdeFile_db_path := abcdeFile_db_dir + "/" + " abcde "
	sha_abcde := RecordFile_TestHelper( t, tmpFilePath_abcde, abcdeFile_db_dir, abcdeFile_db_path, 3, 1)
	assert.Equal( t, sha_abcde, abcde_sha_std)
	// =========================================Check that we can delete entries
	assert.NotEqual( t, sha_empty, sha_hello)
	DeleteFile_TestHelper( t, emptyFile_db_dir, emptyFile_db_path, sha_empty, 2, 0)
	// ========================================Check tat we con overwrite
	tmpFilePath_hello2 := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello 2")
	sha_hello2 := RecordFile_TestHelper( t, tmpFilePath_hello2, helloFile_db_dir, helloFile_db_path, 2, 2)
	assert.NotEqual( t, sha_hello, sha_hello2)
	// =========================================
}

