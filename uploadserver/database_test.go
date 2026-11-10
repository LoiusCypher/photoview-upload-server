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
	"testing"

	"github.com/spf13/afero"
*/
	"os"
	"github.com/loiuscypher/photoview-upload-server/uploadserver" 
)

func TestGetHandler(t *testing.T) {
	// os.Remove("/sqlroot/shafiles.db")
	// var fs afero.Fs
	if _, err := os.Stat("/sqlroot/shafiles.db"); errors.Is(err, os.ErrNotExist) {
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

