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
	"errors"
	"os"
	"testing"
	"github.com/spf13/afero"
)

func TestDatabase(t *testing.T) {
	// os.Remove("/sqlroot/shafiles.db")
	var fs afero.Fs
	fs = afero.NewMemMapFs()
	if err := fs.MkdirAll("/sqlroot", 0755); err != nil {
		t.Fatalf("failed to create database folder: %v", err)
	}
	
	if db := openDb(); db == nil {
		t.Fatalf("init did not create db: %v", err)
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

