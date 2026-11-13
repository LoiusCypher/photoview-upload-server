package uploadserver

import (
/*
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
*/
	//"errors"
	//"net/http/httptest"
	//"os"
	"path"
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
)

func TestDatabase(t *testing.T) {
	CreateDatabase_TestHelper( t)
	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	//var fileInfos []FileInfo
	fileInfos, err := listShas( "")
	assert.Nil( t, err)
	assert.Equal( t, len(fileInfos), 0)
	// =========================================Add a file
	tmpPath := "/tmp"
	err = fs.MkdirAll( path.Dir(tmpPath), 0755)
	assert.Nil( t, err)
	// =========================================Add a file
	tmpFilePath_empty := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "")
	emptyFile_db_dir := "arbitrary / file "
	emptyFile_db_path := emptyFile_db_dir + "/" + " path"
	sha_empty := RecordFile_TestHelper( t, tmpFilePath_empty, emptyFile_db_dir, emptyFile_db_path, 1)
	// =========================================dd a file
	tmpFilePath_hello := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello")
	helloFile_db_dir := "arbitrary / say "
	helloFile_db_path := helloFile_db_dir + "/" + " hello"
	sha_hello := RecordFile_TestHelper( t, tmpFilePath_hello, helloFile_db_dir, helloFile_db_path, 2)
	// =========================================Check that we can delete entries
	assert.NotEqual( sha_empty, sha_hello)
	DeleteFile_TestHelper( t, emptyFile_db_dir, emptyFile_db_path, sha_empty, 1)
	// ========================================Check tat we con overwrite
	tmpFilePath_hello2 := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello 2")
	sha_hello2 := RecordFile_TestHelper( t, tmpFilePath_hello2, helloFile_db_dir, helloFile_db_path, 2)
	assert.NotEqual( t, sha_hello, sha_hello2)
	// =========================================
}

