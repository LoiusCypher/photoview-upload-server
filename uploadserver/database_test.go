package uploadserver

import (
/*
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/afero"
*/
	//"errors"
	"net/http/httptest"
	//"os"
	"testing"
	"github.com/spf13/afero"
)

func TestDatabase(t *testing.T) {
	// sqlPath := "/sqlroot/shafiles.db"
	sqlPath := "/opt/app/shafiles.db"
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll(os.Dir(sqlPath), 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	if *SetDbPath(sqlPath) != "/opt/app" {
		t.Fatalf("set db path failed")
	}
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db")
	}
	if ! afero.Exists(sqlPath); db == nil {
		t.Fatalf("db file was not createb")
	}
	w := httptest.NewRecorder()
	found, err := ListShas( w, "")
	if err != nil {
		t.Fatalf("error reading sha list: %v", err)
	}
	if found {
		t.Fatalf("sha list was expected to be empty first")
	}
}

