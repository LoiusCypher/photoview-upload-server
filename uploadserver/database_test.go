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

	"github.com/spf13/afero"
*/
	//"errors"
	"net/http/httptest"
	"os"
	"path"
	"testing"
	"github.com/spf13/afero"
)

func TestDatabase(t *testing.T) {
	// sqlPath := "/sqlroot/shafiles.db"
	fs := afero.NewMemMapFs()
	sqlPath := "/opt/app/shafiles.db"
	if err := fs.MkdirAll( path.Dir(sqlPath), 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	if *SetDbPath(sqlPath) != "/opt/app" {
		t.Fatalf("set db path failed")
	}
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db")
	}
	found, err := fs.Exists(sqlPath)
	if err != nil {
		t.Fatalf("db file check error: %v", err)
	}
	if ! found {
		t.Fatalf("db file was not createb")
	}
	w := httptest.NewRecorder()
	found, err = ListShas( w, "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if found {
		t.Fatalf("sha list was expected to be empty first")
	}
	tmpPath := "/tmp"
	if err := fs.MkdirAll( path.Dir(sqlPath), 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	f, err := fs.TempFile( "/tmp", "*.pdf")
	var cnt int
	cnt, err := f.WriteString( "")
	if err != nil {
                t.Fatalf("failed to create empty test file: %v", err)
	}
	if cnt != 0 {
                t.Fatalf("empty test file is not empty:")
	}
	emptyFile_db_path := "arbitrary / file / path"
	err = recordSha( emptyFile_path, emptyFile_db_path)
	if err != nil {
		t.Fatalf("error recording sha of empiy file %v", err)
	}
/*
	helloFile_path := "arbitrary / say / hello"
	helloFile_db_path := "arbitrary / say / hello"
	if err := afero.WriteFile(fs, helloFile_path), []byte("Hello"), 0644); err != nil {
                t.Fatalf("failed to create empiy test file: %v", err)
	}
*/
}

