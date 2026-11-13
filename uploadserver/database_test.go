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
	emptyFile_db_path := "arbitrary / file / path"
	shaCalc_empty, err1 := calcSha( tmpFilePath_empty)
	assert.Nil( t, err1)
	shaRec_empty, err2 := recordSha( tmpFilePath_empty, emptyFile_db_path)
	assert.Nil( t, err2)
	assert.Equal( t, shaCalc_empty, shaRec_empty)
	shaGet_empty, err3 = getPathSha( emptyFile_db_path)
	assert.Nil( t, err3)
	assert.Equal( t, shaCalc_empty, shaGet_empty)
	fileInfos, err4 := listShas( "")
	assert.Nil( t, err4)
	assert.Equal( t, len(fileInfos), 1)
	assert.Equal( t, shaCalc_empty, fileInfos.Infos[0].Sha)
	// =========================================dd a file
	tmpFilePath_hello := CreateFile_TestHelper( t, fs, tmpPath, "*.pdf", "Hello")
	helloFile_db_dir := "arbitrary / say "
	helloFile_db_path := helloFile_db_dir + "/" + " hello"
	shaCalc_hello, err5 := calcSha( tmpFilePath_hello)
	assert.Nil( t, err5)
	shaRec_hello, err6 := recordSha( tmpFilePath_hello, helloFile_db_path)
	assert.Nil( t, err6)
	assert.Equal( t, shaCalc_hello, shaRec_hello)
	shaGet_hello, err7 = getPathSha( helloFile_db_path)
	assert.Nil( t, err7)
	assert.Equal( t, shaCalc_hello, shaGet_hello)
	fileInfos, err8 := listShas( "")
	assert.Nil( t, err8)
	assert.Equal( t, len(fileInfos), 2)
	assert.Equal( t, shaCalc_hello, shaGet_hello)
	fileInfos, err8 := listShas( helloFile_db_dir)
	assert.Nil( t, err8)
	assert.Equal( t, len(fileInfos), 1)
	assert.Equal( t, shaCalc_empty, fileInfos.Infos[0].Sha)
	// =========================================
}

