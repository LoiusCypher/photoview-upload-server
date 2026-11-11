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
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	sqlPath := "/opt/app/shafiles.db"
	if err := fs.MkdirAll( path.Dir(sqlPath), 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	dbPath := *SetDbPath(sqlPath)
	if dbPath != sqlPath {
		t.Fatalf("set db path failed %s", dbPath)
	}
	found, err := afero.Exists(fs, sqlPath)
	if err != nil {
		t.Fatalf("db file check error: %v", err)
	}
	if !found {
		t.Error("db file was not created")
	}
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db")
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
	// f.Close()
	emptyFile_db_path := "arbitrary / file / path"
	// var sha string
	_, err = calcSha( tmpFilePath)
	if err != nil {
		t.Errorf("error calculating sha of empty file %v", err)
	}
	err = recordSha( tmpFilePath, emptyFile_db_path)
	if err != nil {
		t.Errorf("error recording sha of empty file %v", err)
	}
	f.Close()

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

