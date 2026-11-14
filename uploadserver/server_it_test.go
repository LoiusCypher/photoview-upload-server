package uploadserver

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"mime/multipart"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/spf13/afero"
)

func TestServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var docRoot string
	var fs afero.Fs
	if v, ok := os.LookupEnv("TEST_WITH_REAL_FS"); ok && v != "" {
		docRoot = v
		fs = afero.NewOsFs()
	} else {
		docRoot = "/tmp"
		sqlRoot = "/tmp"
		fs = afero.NewMemMapFs()
	}

	if err := fs.MkdirAll(docRoot, 0755); err != nil {
		t.Fatalf("failed to create document root: %v", err)
	}
	if err := afero.WriteFile(fs, path.Join(docRoot, "test.txt"), []byte("lorem ipsum"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := fs.Mkdir(path.Join(docRoot, "foo"), 0755); err != nil && !os.IsExist(err) {
		t.Fatalf("failed to create directory: %v", err)
	}
	if err := afero.WriteFile(fs, path.Join(docRoot, "foo", "bar.txt"), []byte("hello, world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	CreateDatabase_TestHelper( t)

	var target string
	if addr, ok := os.LookupEnv("TEST_TARGET_ADDR"); ok && addr != "" {
		target = addr
	} else {
		port, err := getAvailablePort()
		if err != nil {
			t.Fatalf("unable to find an available port: %v", err)
		}
		target = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		config := ServerConfig{
			Addr:            target,
			DocumentRoot:    docRoot,
			EnableCORS:      true,
			MaxUploadSize:   10240,
			ShutdownTimeout: 5000,
		}
		ready := make(chan struct{})
		server := Server{config, afero.NewBasePathFs(fs, docRoot)}
		go func() {
			t.Logf("starting server at %s", target)
			server.Start(ctx, ready) // nolint:errcheck
		}()
		<-ready
	}

	base, err := url.Parse("http://" + target)
	if err != nil {
		t.Fatalf("failed to parse base url: %v", err)
	}

	withPreservingOriginal := func(t *testing.T, path string, f func()) {
		original, err := afero.ReadFile(fs, path)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}
		f()
		if err := afero.WriteFile(fs, path, original, 0644); err != nil {
			t.Fatalf("failed to restore original content: %v", err)
		}
	}

	t.Run("PUT /file?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		CreatePdf_TestHelper( t, []string{ "hello, world",}, pdfFilePath)
		file, err1 := os.Open( pdfFilePath)
		assert.Nil(t, err1)
		defer file.Close()
		sha, err2 := calcSha( pdfFilePath)
		assert.Nil(t, err2)
		req, err3 := makeFormRequest(u, http.MethodPut, pdfFilePath, file)
		assert.Nil(t, err3)
		q := req.URL.Query()
        	q.Add("name", pdfFilePath)
		req.URL.RawQuery = q.Encode()
		//fmt.Println("Req", req)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusCreated)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		body, err5 := io.ReadAll(resp.Body)
		assert.Nil(t, err5, fmt.Sprintf("failed to read response body: %v", err5))
		var result SuccessFileInfosResult
		err6 := json.Unmarshal(body, &result)
		assert.Nil(t, err6, fmt.Sprintf("failed to decode response body: %v", err6))
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, sha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join(docRoot, "hello-01.jpg"))
	})

	t.Run("POST /file should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "foo.pdf"
		CreatePdf_TestHelper( t, []string{ "hello, world",}, pdfFilePath)
		file, err1 := os.Open( pdfFilePath)
		assert.Nil(t, err1)
		req, err3 := makeFormRequest(u, http.MethodPost, pdfFilePath, file)
		assert.Nil(t, err3)
		q := req.URL.Query()
        	q.Add("name", pdfFilePath)
		req.URL.RawQuery = q.Encode()
		defer file.Close()
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to POST: %v", err4))
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusMethodNotAllowed)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		body, err5 := io.ReadAll(resp.Body)
		assert.Nil(t, err5, fmt.Sprintf("failed to read response body: %v", err5))
		var result ErrorResult
		err6 := json.Unmarshal(body, &result)
		assert.Nil(t, err6, fmt.Sprintf("failed to decode response body: %v", err6))
		expected := ErrorResult{false, "POST is not allowed on /file"}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})
/*
	t.Run("POST /upload should fail due to duplication", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "test.txt")
		localOriginal, err := afero.ReadFile(fs, localPath)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}

		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "test.txt", bytes.NewBufferString("hello, new world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusConflict)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "the file already exists"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, fs, localPath, localOriginal)
	})
*/
	t.Run("PUT /file?name=hello.pdf should succeed even on overwrite", func(t *testing.T) {
		pdfFilePath := "hello.pdf"

		CreatePdf_TestHelper( t, []string{ "hello, world",}, pdfFilePath)
		file, err1 := os.Open( pdfFilePath)
		assert.Nil(t, err1)
		defer file.Close()
		shaOld, err2 := calcSha( pdfFilePath)
		assert.Nil(t, err2)

		helloFile_db_dir := "arbitrary / file "
		helloFile_db_path := helloFile_db_dir + "/" + " path"
		sha_hello := RecordFile_TestHelper( t, pdfFilePath, helloFile_db_dir, helloFile_db_path, 1, 1)
		assert.Equal( t, shaOld, sha_hello)

		localPath := filepath.Join(docRoot, "test.txt")
		withPreservingOriginal(t, localPath, func() {
			u := base.JoinPath("/file")
			q := u.Query()
			q.Set("name", helloFile_db_path)
			u.RawQuery = q.Encode()

			newContent := "hello, new world"
			CreatePdf_TestHelper( t, []string{ newContent,}, pdfFilePath)
			shaNew, err2 := calcSha( pdfFilePath)
			assert.NotEqual( t, sha_hello, shaNew)
			assert.Nil(t, err2)
			req, err3 := makeFormRequest(u, http.MethodPut, pdfFilePath, file)
			assert.Nil(t, err3)
			resp, err4 := http.DefaultClient.Do(req)
			assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
			defer resp.Body.Close()
			assert.Equal(t, resp.StatusCode, http.StatusCreated)
			assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")

			body, err5 := io.ReadAll(resp.Body)
			assert.Nil(t, err5, fmt.Sprintf("failed to read response body: %v", err5))
			var result SuccessFileInfosResult
			err6 := json.Unmarshal(body, &result)
			assert.Nil(t, err6, fmt.Sprintf("failed to decode response body: %v", err6))
			expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, shaNew},},}
			assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
			assert.FileExists(t, filepath.Join(docRoot, "hello-01.jpg"))
		})
	})
/*
	t.Run("PUT /files/hello_put.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPut, "hello.txt", content)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello_put.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, fs, filepath.Join(docRoot, "hello_put.txt"), []byte("hello, world"))
	})
*/ /*
	t.Run("PUT /files/hello/world.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello/world.txt")
		req, err := makeFormRequest(u, http.MethodPut, "world.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		if !result.OK {
			t.Errorf("result.OK = false, want = true")
		}
		if result.Path != "/files/hello/world.txt" {
			t.Errorf("path = %s, want = \"/files/hello/world.txt\"", result.Path)
		}
		if exists, err := afero.DirExists(fs, filepath.Join(docRoot, "hello")); err != nil {
			t.Fatalf("failed to check if directory exists: %v", err)
		} else if !exists {
			t.Errorf("directory /hello does not exist")
		}
		verifyLocalFile(t, fs, filepath.Join(docRoot, "hello", "world.txt"), []byte("hello, world"))
	})
*/ /*
	t.Run("PUT /upload should fail", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMethodNotAllowed {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusMethodNotAllowed)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "PUT is not allowed on /upload"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})
*/ /*
	t.Run("PUT /files/foo/bar.txt should fail (conflict)", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("new world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusConflict {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusConflict)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "the file already exists"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})
*/ /*
	t.Run("PUT /files/foo/bar.txt should succeed (overwrite)", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "foo", "bar.txt")
		withPreservingOriginal(t, localPath, func() {
			u := base.JoinPath("/files/foo/bar.txt")
			q := u.Query()
			q.Set("overwrite", "true")
			u.RawQuery = q.Encode()

			req, err := makeFormRequest(u, http.MethodPut, "foo.txt", bytes.NewBufferString("new world"))
			if err != nil {
				t.Fatal(err)
			}
			resp, err := http.DefaultClient.Do(req)
			if err != nil {
				t.Fatalf("failed to PUT: %v", err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusCreated {
				t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
			}
			if resp.Header.Get("Content-Type") != "application/json" {
				t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
			}
			body, err := io.ReadAll(resp.Body)
			if err != nil {
				t.Fatalf("failed to read response body: %v", err)
			}
			var result SuccessfullyUploadedResult
			if err := json.Unmarshal(body, &result); err != nil {
				t.Fatalf("failed to decode response body: %v", err)
			}
			expected := SuccessfullyUploadedResult{true, "/files/foo/bar.txt"}
			if !reflect.DeepEqual(result, expected) {
				t.Errorf("result = %+v, want = %+v", result, expected)
			}
			verifyLocalFile(t, fs, filepath.Join(docRoot, "foo", "bar.txt"), []byte("new world"))
		})
	})
*/
	t.Run("GET /file?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		req, err1 := http.NewRequest( http.MethodGet, u, nil)
		assert.Nil(t, err1)
		q := req.URL.Query()
        	q.Add("name", pdfFilePath)
		req.URL.RawQuery = q.Encode()
		//fmt.Println("Req", req)
		resp, err4 := http.DefaultClient.Do(req)
		//resp, err := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusOK)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		body, err5 := io.ReadAll(resp.Body)
		assert.Nil(t, err5, fmt.Sprintf("failed to read response body: %v", err5))
		var result SuccessFileInfosResult
		err6 := json.Unmarshal(body, &result)
		assert.Nil(t, err6, fmt.Sprintf("failed to decode response body: %v", err6))
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, sha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})
/*
	t.Run("GET /files/foo/bar/baz.txt should fail (not found)", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar/baz.txt")
		resp, err := http.Get(u.String())
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNotFound)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "file not found"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})
*/
	t.Run("OPTIONS /list", func(t *testing.T) {
		u := base.JoinPath("/list")
		req, err1 := http.NewRequest(http.MethodOptions, u.String(), nil)
		assert.Nil(t, err1)
		resp, err2 := http.DefaultClient.Do(req)
		assert.Nil(t, err2)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusNoContent)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		assert.NotEqual(t, acam, "")
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{ "GET",}
		assert.True( t, containsAll(allowedMethods, expectedAllowedMethods), fmt.Sprintf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods))
		assert.Equal( t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
	})

	t.Run("OPTIONS /file", func(t *testing.T) {
		u := base.JoinPath("/file")
		req, err1 := http.NewRequest(http.MethodOptions, u.String(), nil)
		assert.Nil(t, err1)
		resp, err2 := http.DefaultClient.Do(req)
		assert.Nil(t, err2)
		defer resp.Body.Close()
		assert.Equal(t, resp.StatusCode, http.StatusNoContent)
		assert.Equal(t, resp.Header.Get("Content-Type"), "application/json")
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		assert.NotEqual(t, acam, "")
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{ "PUT", "GET", "DELETE",}
		assert.True( t, containsAll(allowedMethods, expectedAllowedMethods), fmt.Sprintf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods))
		assert.Equal( t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
	})
}
/*
func TestServerWithAuth(t *testing.T) {
	docRoot := "/opt/app"

	fs := afero.NewMemMapFs()
	if err := fs.MkdirAll(docRoot, 0755); err != nil {
		t.Fatalf("failed to create document root: %v", err)
	}
	if err := afero.WriteFile(fs, path.Join(docRoot, "test.txt"), []byte("lorem ipsum"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	if err := afero.WriteFile(fs, path.Join(docRoot, "foo", "bar.txt"), []byte("hello, world"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	port, err := getAvailablePort()
	if err != nil {
		t.Fatalf("unable to find an available port: %v", err)
	}

	roToken := "read-only-token"
	rwToken := "read-write-token"
	addr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
	config := ServerConfig{
		Addr:            addr,
		DocumentRoot:    docRoot,
		EnableCORS:      true,
		MaxUploadSize:   16,
		ShutdownTimeout: 5000,
		EnableAuth:      true,
		ReadOnlyTokens:  []string{roToken},
		ReadWriteTokens: []string{rwToken},
	}
	ready := make(chan struct{})
	server := Server{config, afero.NewBasePathFs(fs, docRoot)}
	go func() {
		t.Logf("starting server at %s", addr)
		server.Start(ctx, ready) // nolint:errcheck
	}()
	<-ready

	base, err := url.Parse("http://" + addr)
	if err != nil {
		t.Fatalf("failed to parse base url: %v", err)
	}
	t.Run("POST /upload token via header", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+rwToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", ct)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, fs, filepath.Join(docRoot, "hello.txt"), []byte("hello, world"))
	})

	t.Run("POST /upload token via query", func(t *testing.T) {
		u := base.JoinPath("/upload")
		q := u.Query()
		q.Set("token", rwToken)
		u.RawQuery = q.Encode()

		req, err := makeFormRequest(u, http.MethodPost, "hello_query.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", ct)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello_query.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, fs, filepath.Join(docRoot, "hello_query.txt"), []byte("hello, world"))
	})

	t.Run("POST /upload with read-only token", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", ct)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "unauthorized"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("POST /upload without token", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", ct)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "unauthorized"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("PUT /files/hello_put.txt with read-write token", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPut, "hello.txt", content)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+rwToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusCreated)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result SuccessfullyUploadedResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := SuccessfullyUploadedResult{true, "/files/hello_put.txt"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
		verifyLocalFile(t, fs, filepath.Join(docRoot, "hello_put.txt"), []byte("hello, world"))
	})

	t.Run("PUT /files/hello_put.txt with read-only token", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPut, "hello.txt", content)
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "unauthorized"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("PUT /files/hello_put.txt without tokens", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPut, "hello.txt", content)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to PUT: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
		if resp.Header.Get("Content-Type") != "application/json" {
			t.Errorf("Content-Type = %s, want = \"application/json\"", resp.Header.Get("Content-Type"))
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "unauthorized"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("GET /files/foo/bar.txt using rw token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create GET request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rwToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if string(body) != "hello, world" {
			t.Errorf("body = %s, want = \"hello, world\"", body)
		}
	})

	t.Run("GET /files/foo/bar.txt using ro token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create GET request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if string(body) != "hello, world" {
			t.Errorf("body = %s, want = \"hello, world\"", body)
		}
	})

	t.Run("GET /files/foo/bar.txt using ro token with query parameter", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		q := u.Query()
		q.Set("token", roToken)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create GET request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		if string(body) != "hello, world" {
			t.Errorf("body = %s, want = \"hello, world\"", body)
		}
	})

	t.Run("GET /files/foo/bar.txt without tokens", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create GET request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Fatalf("failed to read response body: %v", err)
		}
		var result ErrorResult
		if err := json.Unmarshal(body, &result); err != nil {
			t.Fatalf("failed to decode response body: %v", err)
		}
		expected := ErrorResult{false, "unauthorized"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})

	t.Run("HEAD /files/foo/bar.txt using rw token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodHead, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create HEAD request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rwToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to GET: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("HEAD /files/foo/bar.txt using ro token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodHead, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create HEAD request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to HEAD: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("HEAD /files/foo/bar.txt using ro token with query parameter", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		q := u.Query()
		q.Set("token", roToken)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodHead, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create HEAD request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to HEAD: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("HEAD /files/foo/bar.txt without tokens", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodHead, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create HEAD request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to HEAD: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusUnauthorized {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusUnauthorized)
		}
	})

	t.Run("OPTIONS /files/foo/bar.txt using rw token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+rwToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to OPTIONS: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"GET", "HEAD", "PUT"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})

	// check the authentication does not affect the response of OPTIONS.
	t.Run("OPTIONS /files/foo/bar.txt using ro token with Authorization header", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to OPTIONS: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"GET", "HEAD", "PUT"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})

	t.Run("OPTIONS /files/foo/bar.txt using ro token with query parameter", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		q := u.Query()
		q.Set("token", roToken)
		u.RawQuery = q.Encode()

		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		req.Header.Set("Authorization", "Bearer "+roToken)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to OPTIONS: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"GET", "HEAD", "PUT"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})

	t.Run("OPTIONS /files/foo/bar.txt without tokens", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := http.NewRequest(http.MethodOptions, u.String(), nil)
		if err != nil {
			t.Fatalf("failed to create OPTIONS request: %v", err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to OPTIONS: %v", err)
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNoContent {
			t.Errorf("status = %d, want = %d", resp.StatusCode, http.StatusNoContent)
		}
		acam := resp.Header.Get("Access-Control-Allow-Methods")
		if acam == "" {
			t.Errorf("Access-Control-Allow-Methods got empty, want not empty")
		}
		allowedMethods := strings.Split(acam, ",")
		for i := range allowedMethods {
			allowedMethods[i] = strings.TrimSpace(allowedMethods[i])
		}
		expectedAllowedMethods := []string{"GET", "HEAD", "PUT"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})
}
*/ /*
func verifyLocalFile(t *testing.T, fs afero.Fs, path string, content []byte) {
	got, err := afero.ReadFile(fs, path)
	if err != nil {
		t.Fatalf("failed to read local file: %v", err)
	}
	if !reflect.DeepEqual(got, content) {
		t.Errorf("local file content = %s, want = %s", got, content)
	}
}
*/
// containsAll reports whether a contains all elements of b.
func containsAll[T comparable](a, b []T) bool {
	for _, x := range b {
		if !slices.Contains(a, x) {
			return false
		}
	}
	return true
}

// makeFormRequest creates a new http.Request with multipart/form-data.
func makeFormRequest(url *url.URL, method, filename string, content io.Reader) (*http.Request, error) {
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	written, err := io.Copy(fw, content)
	if err != nil {
		return nil, err
	}
	if written == 0 {
		return nil, fmt.Errorf("no bytes written")
	}
	w.Close()

	req, err := http.NewRequest(method, url.String(), b)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, nil
}

// getAvailablePort returns an available port number randomly.
func getAvailablePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	listener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, err
	}
	defer listener.Close()
	return listener.Addr().(*net.TCPAddr).Port, nil
}
