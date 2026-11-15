package uploadserver

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/afero"
)

func TestGetHandler(t *testing.T) {
	var sqlroot string
	sqlroot = "/sqlroot"
	if err := os.MkdirAll(sqlroot, 0755); err != nil {
		t.Fatalf("failed to create sql root: %v", err)
	}
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
/*
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
			body: "hello, world",
		},
*/
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
			body: `{"ok":false,"error":"file not found"}`,
		},
		{
			name: "get without endpoint",
			args: args{
				Method: http.MethodGet,
				Url:    "/abc",
			},
			want: http.StatusNotFound,
			body: `{"ok":false,"error":"file not found"}`,
		},
/*
		{
			name: "get directory",
			args: args{
				Method: http.MethodGet,
				Url:    "/list/foo",
			},
			want: http.StatusNotFound,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":false,"error":"foo is a directory"}`,
		},
*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfRoot := "/tnp"
			// _ := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfRoot)
			config := ServerConfig{
				PictureRoot:  "/opt/app",
				EnableCORS:   true,
			}
			server := Server{config, }
			req, err := http.NewRequest(tt.args.Method, tt.args.Url, nil)
			if err != nil {
				t.Fatal(err)
			}

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleFileGet))
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.want {
				t.Errorf("status = %d, want = %d", status, tt.want)
			}
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
	jpgRoot := "/opt/app"
	type args struct {
		Method  string
		Url     string
		Content []byte
		Name    string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
/*
		{
			name: "PUT /files/hello.txt with text",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/hello.txt",
				Content: []byte("hello, world"),
				Name:    "hello.txt",
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
				Url:     "/files/empty",
				Content: []byte{},
				Name:    "empty",
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
				Url:     "/files/hello/world.txt",
				Content: []byte("hello, world"),
				Name:    "world.txt",
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
				Url:     "/files/",
				Content: []byte("hello"),
				Name:    "hello",
			},
			want: http.StatusMethodNotAllowed,
			body: `{"ok":false,"error":"PUT is accepted on /files/:name"}`,
		},
		{
			name: "PUT large file should fail",
			args: args{
				Method:  http.MethodPut,
				Url:     "/files/toolarge",
				Content: []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZ"),
				Name:    "toolarge",
			},
			want: http.StatusRequestEntityTooLarge,
			body: `{"ok":false,"error":"file size limit exceeded"}`,
		},
		// TODO: add text without name
*/
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := os.MkdirAll(jpgRoot, 0755); err != nil {
				t.Fatal(err)
			}
			config := ServerConfig{
				PictureRoot:  jpgRoot,
				EnableCORS:    true,
				MaxUploadSize: 16,
			}
			server := Server{config, }

			b := new(bytes.Buffer)
			w := multipart.NewWriter(b)
			fw, err := w.CreateFormFile("file", tt.args.Name)
			if err != nil {
				t.Fatal(err)
			}
			written, err := fw.Write(tt.args.Content)
			if err != nil {
				t.Fatal(err)
			}
			if written == 0 && len(tt.args.Content) > 0 {
				t.Fatalf("content has %d bytes but no bytes written", len(tt.args.Content))
			}
			w.Close()

			req, err := http.NewRequest(tt.args.Method, tt.args.Url, b)
			if err != nil {
				t.Fatal(err)
			}
			req.Header.Set("Content-Type", w.FormDataContentType())

			rr := httptest.NewRecorder()
			handler := http.HandlerFunc(server.handle(server.handleFilePut))
			handler.ServeHTTP(rr, req)

			if status := rr.Code; status != tt.want {
				t.Errorf("status = %d, want = %d", status, tt.want)
			}
			if body := rr.Body.String(); body != tt.body {
				t.Errorf("body = \"%s\", want = \"%s\"", body, tt.body)
			}
			if rr.Code == http.StatusCreated || rr.Code == http.StatusOK {
				var resp struct {
					OK   bool   `json:"ok"`
					Path string `json:"path"`
				}
				if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
					t.Errorf("unexpected response body: %v", err)
				}
				localPath := strings.TrimPrefix(resp.Path, "/files/")
				localFullPath := filepath.Join(jpgRoot, localPath)
				t.Logf("opening %s to verify", localFullPath)
				f, err := os.Open(localFullPath)
				if err != nil {
					t.Errorf("failed to verify local file. cannot open %s: %v", localFullPath, err)
					return
				}
				defer f.Close()
				uploaded, err := afero.ReadAll(f)
				if err != nil {
					t.Errorf("failed to verify local file. cannot read %s: %v", localFullPath, err)
					return
				}
				if !reflect.DeepEqual(uploaded, tt.args.Content) {
					t.Errorf("failed to verify. request body = %v, local file = %v", tt.args.Content, uploaded)
				}
			}
			for k, v := range tt.headers {
				if rr.Header().Get(k) != v {
					t.Errorf("header %s = %s, want %s", k, rr.Header().Get(k), v)
				}
			}
		})
	}
}

