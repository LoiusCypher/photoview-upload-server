package uploadserver

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHandler(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	fooBar_db_path := "foo/bar.pdf"
	fooBar_db_sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_ = RecordPdfSha_TestHelper( t, fooBar_db_sha, fooBar_db_path)

	type args struct {
		Method string
		Url    string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
		{
			name: "get existing file",
			args: args{
				Method: http.MethodGet,
				Url:    "/file?name=foo/bar.pdf",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]}`,
		},
		{
			name: "get non-existing file",
			args: args{
				Method: http.MethodGet,
				Url:    "/file?name=bar/baz",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"not found"}`,
		},
		{
			name: "get without endpoint",
			args: args{
				Method: http.MethodGet,
				Url:    "/abc",
			},
			want: http.StatusNotFound,
			body: `{"ok":false,"error":"not found"}`,
		},
		{
			name: "get directory",
			args: args{
				Method: http.MethodGet,
				Url:    "/list?name=foo",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"not found"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// pdfRoot := "/tnp"
			// _ := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfRoot)
			config := ServerConfig{
				PictureRoot:  "/opt/app",
				EnableCORS:   true,
			}
			server := Server{config, }
			req, err := http.NewRequest(tt.args.Method, tt.args.Url, nil)
			assert.Nil( t, err)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleFileGet))
			handler.ServeHTTP(rr, req)

			status := rr.Code
			assert.Equal( t, tt.want, status, fmt.Sprintf("status = %d, want = %d", status, tt.want))
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
			for k, v := range tt.headers {
				if rr.Header().Get(k) != v {
					t.Errorf("header %s = %s, want %s", k, rr.Header().Get(k), v)
				}
			}
		})
	}
}

func TestServer_PutHandler(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err0 := os.MkdirTemp("", "")
	require.Nil( t, err0)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	pdfRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( pdfRoot)

	jpgRoot, err3 := os.MkdirTemp("", "")
	require.Nil(t, err3)
	defer os.RemoveAll( jpgRoot)

	type args struct {
		Method  string
		Url     string
		Content []string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
		{
			name: "PUT /file?name=hello.pdf with text",
			args: args{
				Method:  http.MethodPut,
				Url:     "/file?name=hello.txt",
				Content: []string{ "hello, world",},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"path":"/files/hello.txt"}`,
		},
		{
			name: "PUT /files/empty with an empty content",
			args: args{
				Method:  http.MethodPut,
				Url:     "/file?name=empty.pdf",
				Content: []string{},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"path":"/files/empty"}`,
		},
		{
			name: "PUT /files/hello/world.txt will create directory and file",
			args: args{
				Method:  http.MethodPut,
				Url:     "/file?name=hello/world.pdf",
				Content: []string{ "hello, world",},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"path":"/files/hello/world.txt"}`,
		},
		{
			name: "PUT /files/ should fail",
			args: args{
				Method:  http.MethodPut,
				Url:     "/file",
				Content: []string{ "hello",},
			},
			want: http.StatusMethodNotAllowed,
			body: `{"ok":false,"error":"PUT is accepted on /files/:name"}`,
		},
		{
			name: "PUT large file should fail",
			args: args{
				Method:  http.MethodPut,
				Url:     "/file?=toolarge.pdf",
				Content: []string{ "ABCDEFGHIJKLMNOPQRSTUVWXYZ", "Second Page", "Last Page",},
			},
			want: http.StatusRequestEntityTooLarge,
			body: `{"ok":false,"error":"file size limit exceeded"}`,
		},
		// TODO: add text without name
/*
*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(jpgRoot, 0755); err != nil {
				t.Fatal(err)
			}
			config := ServerConfig{
				PictureRoot:   jpgRoot,
				EnableCORS:    true,
				MaxUploadSize: 3000,
			}
			server := Server{config, }

			pdfFilePath := CreatePdfFile_TestHelper( t, tt.args.Content, pdfRoot)
			// sha, err0 := calcSha( pdfFilePath)
			// require.Nil(t, err0)
			file, err1 := os.Open( pdfFilePath)
			assert.Nil(t, err1)
			defer file.Close()
			b := new(bytes.Buffer)
			w := multipart.NewWriter(b)
			defer w.Close()
			fw, err2 := w.CreateFormFile("file", pdfFilePath)
			require.Nil(t, err2)
			// defer fw.Close()
			written, err3 := io.Copy(fw, file)
			assert.Nil(t, err3)
			// assert.NotEqual(t, 0, written, fmt.Sprintf("no bytes written"))
			assert.NotEqual(t, 0, written, "no bytes written")
			w.Close()
			req, err4 := http.NewRequest(tt.args.Method, tt.args.Url, b)
			assert.Nil(t, err4)
			req.Header.Set("Content-Type", w.FormDataContentType())

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleFilePut))
			handler.ServeHTTP(rr, req)

			status := rr.Code
			assert.Equal( t, tt.want, status, fmt.Sprintf("status = %d, want = %d", status, tt.want))
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
			if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
				var resp SuccessFileInfosResult
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("unexpected response body: %v", err)
				}
				localPath := strings.TrimSuffix(resp.Files[0].Path, ".pdf") + "-01.jpg"
				localFullPath := filepath.Join(jpgRoot, localPath)
				assert.FileExists( t, localFullPath)
				t.Logf("opening %s to verify", localFullPath)
			}
			for k, v := range tt.headers {
				if rr.Header().Get(k) != v {
					t.Errorf("header %s = %s, want %s", k, rr.Header().Get(k), v)
				}
			}
		})
	}
}

