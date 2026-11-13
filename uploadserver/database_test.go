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
	_ = CreateDatabase_TestHelper( t)
	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
/*
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
*/
	//var fileInfos []FileInfo
	fileInfos, err := listShas( "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if len(fileInfos) !=0 {
		t.Fatalf("sha list was expected to be empty first")
	}
	tmpPath := "/tmp"
	if err := fs.MkdirAll( path.Dir(tmpPath), 0755); err != nil {
		t.Fatalf("failed to create temp folder: %v", err)
	}
	// =========================================Add a file
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
	var shaCalc string
	shaCalc, err = calcSha( tmpFilePath)
	if err != nil {
		t.Errorf("error calculating sha of empty file %v", err)
	}
	var shaRec string
	shaRec, err = recordSha( tmpFilePath, emptyFile_db_path)
	if err != nil {
		t.Errorf("error recording sha of empty file %v", err)
	}
	if shaCalc != shaRec {
		t.Errorf("sha shoul be same %s %s", shaCalc, shaRec)
	}
	fileInfos, err = listShas( "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if len(fileInfos) != 1 {
		t.Fatalf("sha list was expected to have an entry")
	}
	// =========================================dd a file
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
	_, err = recordSha( tmpFilePath, helloFile_db_path)
	if err != nil {
		t.Fatalf("error recording sha of hello file %v", err)
	}
	var sha string
	sha, err = getPathSha( helloFile_db_path)
	if err != nil {
		t.Fatalf("error getting sha of empty file %v", err)
	}
	if sha == "" {
		t.Fatalf("sha getting was expected to have an entry")
	}
	fileInfos, err = listShas( "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if len(fileInfos) != 2 {
		t.Fatalf("sha list was expected to have two entries")
	}
	// =========================================
}

