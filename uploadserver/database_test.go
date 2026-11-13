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
	sha_empty := RecordFile_TestHelper( t, tmpFilePath_empty, emptyFile_db_path, helloFile_db_dir, 1)
	// =========================================dd a file
	tmpFilePath_hello := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello")
	helloFile_db_dir := "arbitrary / say "
	helloFile_db_path := helloFile_db_dir + "/" + " hello"
	sha_hello := RecordFile_TestHelper( t, tmpFilePath_hello, helloFile_db_path, helloFile_db_dir, 2)
	// =========================================
	assert.NotEqual( sha_empty, sha_hello)
}

