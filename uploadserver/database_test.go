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
	//"os"
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
	found, err := afero.Exists(fs, sqlPath)
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
	// w.Close()
	tmpPath := "/tmp"
	if err := fs.MkdirAll( path.Dir(tmpPath), 0755); err != nil {
		t.Fatalf("failed to create temp folder: %v", err)
	}
	f, err2 := afero.TempFile( fs, tmpPath, "*.pdf")
	if err2 != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	var cnt int
	cnt, err = f.WriteString( "")
	if err != nil {
                t.Fatalf("failed to create empty test file: %v", err)
	}
	if cnt != 0 {
                t.Fatalf("empty test file is not empty:")
	}
	tmpFilePath := f.Name()
	f.Close()
	emptyFile_db_path := "arbitrary / file / path"
	err = recordSha( tmpFilePath, emptyFile_db_path)
	if err != nil {
		t.Fatalf("error recording sha of empty file %v", err)
	}
	w = httptest.NewRecorder()
	found, err = ListShas( w, "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if ! found {
		t.Fatalf("sha list was expected to have an entry")
	}
	// w.Close()
	f, err = afero.TempFile( fs, tmpPath, "*.pdf")
	if err != nil {
		t.Fatalf("error creating temp file: %v", err)
	}
	cnt, err = f.WriteString( "Hello")
	if err != nil {
                t.Fatalf("failed to create hello test file: %v", err)
	}
	if cnt != 5 {
                t.Fatalf("empty hello file is not empty right size")
	}
	tmpFilePath = f.Name()
	f.Close()
	helloFile_db_path := "arbitrary / say / hello"
	err = recordSha( tmpFilePath, helloFile_db_path)
	if err != nil {
		t.Fatalf("error recording sha of hello file %v", err)
	}
}

