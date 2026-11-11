package uploadserver

import (
/*
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path"
	"path/filepath"
	"reflect"
	"strings"

	"github.com/spf13/afero"
*/
	//"errors"
	//"os"
	"testing"
	"github.com/spf13/afero"
)

func TestDatabase(t *testing.T) {
	// os.Remove("/sqlroot/shafiles.db")
	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll("/opt/app", 0755); err != nil {
	//if err := fs.MkdirAll("/sqlroot", 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	if "/opt/app" != SetDbPath("/opt/app") {
		t.Fatalf("set db path failed")
	}
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db")
	}

/*
	var sqlroot string
	sqlroot = "/sqlroot"
	fs = afero.NewMemMapFs()
	if err := fs.MkdirAll(sqlroot, 0755); err != nil {
		t.Fatalf("failed to create sql root: %v", err)
	}
*/
}

