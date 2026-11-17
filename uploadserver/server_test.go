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

func TestGetFileHandler(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	fooBar_db_path := "foo/bar/stuff.pdf"
	fooBar_db_sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_ = RecordPdfSha_TestHelper( t, fooBar_db_sha, fooBar_db_path)

	hello_db_path := "world/hello.pdf"
	hello_db_sha := "68fb1f3ef9c02d42c9c655a93febdc7a63f2a6eee4141a938efbede5413c4601"
	_ = RecordPdfSha_TestHelper( t, hello_db_sha, hello_db_path)

	type args struct {
		query   string
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
				query:  "?name=foo/bar/stuff.pdf",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar/stuff.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]}`,
		},
		{
			name: "get non-existing file",
			args: args{
				query:  "?name=bar/baz",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"not found"}`,
		},
		{
			name: "get without query",
			args: args{
				query:  "",
			},
			want: http.StatusBadRequest,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"missing name of file: /file?name="}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{
				EnableCORS:   true,
			}
			server := Server{config, }
			req, err := http.NewRequest( http.MethodGet, "/file" + tt.args.query, nil)
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

func TestGetListHandler(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	fooBar_db_path := "foo/bar/stuff.pdf"
	fooBar_db_sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_ = RecordPdfSha_TestHelper( t, fooBar_db_sha, fooBar_db_path)

	hello_db_path := "world/hello.pdf"
	hello_db_sha := "68fb1f3ef9c02d42c9c655a93febdc7a63f2a6eee4141a938efbede5413c4601"
	_ = RecordPdfSha_TestHelper( t, hello_db_sha, hello_db_path)

	type args struct {
		query  string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
		{
			name: "get /list?name=foo/bar list directory with ons file",
			args: args{
				query:  "?name=foo/bar",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar/stuff.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]}`,
		},
		{
			name: "get /list?name=foo list directory with subdir content",
			args: args{
				query:  "?name=foo",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar/stuff.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]}`,
		},
		{
			name: "get /list all files",
			args: args{
				query:  "",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar/stuff.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"},{"path":"world/hello.pdf","sha":"68fb1f3ef9c02d42c9c655a93febdc7a63f2a6eee4141a938efbede5413c4601"}]}`,
		},
		{
			name: "get /list?name=bar empty directory",
			args: args{
				query:  "?name=bar",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"file not found"}`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := ServerConfig{
				EnableCORS:   true,
			}
			server := Server{config, }
			req, err := http.NewRequest( http.MethodGet, "/list" + tt.args.query, nil)
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

func TestDeleteFileHandler(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	fooBar_db_path := "foo/bar/stuff.pdf"
	fooBar_db_sha := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	_ = RecordPdfSha_TestHelper( t, fooBar_db_sha, fooBar_db_path)

	hello_db_path := "world/hello.pdf"
	hello_db_sha := "68fb1f3ef9c02d42c9c655a93febdc7a63f2a6eee4141a938efbede5413c4601"
	_ = RecordPdfSha_TestHelper( t, hello_db_sha, hello_db_path)

	type args struct {
		query   string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
		{
			name: "delete existing file",
			args: args{
				query:  "?name=foo/bar/stuff.pdf",
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"foo/bar/stuff.pdf","sha":"e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}]}`,
		},
		{
			name: "delete non-existing file",
			args: args{
				query:  "?name=bar/baz",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"not found"}`,
		},
		{
			name: "delete without query",
			args: args{
				query:  "",
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
			config := ServerConfig{
				EnableCORS:   true,
			}
			server := Server{config, }
			req, err := http.NewRequest( http.MethodDelete, "/file" + tt.args.query, nil)
			assert.Nil( t, err)

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleFileDelete))
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

func TestServer_PostHandler(t *testing.T) {
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
		Path    string
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
			name: "POST /upload hello.pdf with text",
			args: args{
				Path:    "hello.pdf",
				Content: []string{ "hello, world",},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"hello.pdf","sha":"%s"}]}`,
		},
		{
			name: "POST /upload empty.pdf with an empty content",
			args: args{
				Path:    "empty.pdf",
				Content: []string{},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"empty.pdf","sha":"%s"}]}`,
		},
		{
			name: "POST /upload hello/world.pdf will create directory and file",
			args: args{
				Path:    "hello/world.pdf",
				Content: []string{ "hello, world",},
			},
			want: http.StatusCreated,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"hello/world.pdf","sha":"%s"}]}`,
		},
		// TODO: add text without name
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(jpgRoot, 0755); err != nil {
				t.Fatal(err)
			}
			config := ServerConfig{
				PictureRoot:   jpgRoot,
				EnableCORS:    true,
				MaxUploadSize: 1250,
			}
			server := Server{config, }

			pdfFilePath := CreatePdfFile_TestHelper( t, tt.args.Content, pdfRoot)
			sha, err0 := calcSha( pdfFilePath)
			require.Nil(t, err0)
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
			err4 := w.WriteField( "name", tt.args.Path)
			assert.Nil(t, err4)
			w.Close()
			req, err5 := http.NewRequest( http.MethodPost, "/upload", b)
			assert.Nil(t, err5)
			req.Header.Set("Content-Type", w.FormDataContentType())

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handlePostUpload))
			handler.ServeHTTP(rr, req)

			status := rr.Code
			assert.Equal( t, tt.want, status, fmt.Sprintf("status = %d, want = %d", status, tt.want))
			sha_body := strings.Replace( tt.body, "%s", sha, 1)
			if body := rr.Body.String(); body != sha_body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, sha_body)
			}
			if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
				var resp SuccessFileInfosResult
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("unexpected response body: %v", err)
				}
				resp.Files[0].Sha = sha
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

