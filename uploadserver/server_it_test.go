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
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func GenerateUrl_TestHelper(t *testing.T, u *url.URL, filePath string) {
	q := url.Values{}
        q.Add("name", filePath)
	u.RawQuery = q.Encode()
}

func GenerateRequest_TestHelper(t *testing.T, method string, u *url.URL, filePath string) *http.Request {
	q := url.Values{}
        q.Add("name", filePath)
	u.RawQuery = q.Encode()
	req, err1 := http.NewRequest( method, u.String(), nil)
	assert.Nil(t, err1)
	return req
}

func CheckResponseCode_TestHelper(t *testing.T, resp *http.Response, statusCode int) {
	assert.Equal(t, resp.StatusCode, resp.StatusCode)
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func CheckResponse_TestHelper(t *testing.T, resp *http.Response, statusCode int) []byte {
	CheckResponseCode_TestHelper( t, resp, statusCode)
	body, err1 := io.ReadAll(resp.Body)
	assert.Nil(t, err1, fmt.Sprintf("failed to read response body: %v", err1))
	return body
}

func CheckResponseSuccess_TestHelper(t *testing.T, resp *http.Response, statusCode int) SuccessFileInfosResult {
	body := CheckResponse_TestHelper( t, resp, statusCode)
	var result SuccessFileInfosResult
	err2 := json.Unmarshal(body, &result)
	assert.Nil(t, err2, fmt.Sprintf("failed to decode response body: %v", err2))
	return result
}


func TestServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// ghostscript output has to be on real file system
	jpgRoot, err1 := os.MkdirTemp( "", "")
	require.Nil(t, err1))
	defer os.RemoveAll( jpgRoot)

	pdfRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2))
	defer os.RemoveAll( pdfRoot)
	_ = CreatePdfFile_TestHelper( t, []string{ "lorem ipsum",}, pdfRoot)
	pdfPath_hello := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfRoot)

	// sqlite database has to be on real file system or in memory
	// var sqlRoot := os.MkdirTemp("", "")
	// defer os.RemoveAll( sqlRoot)
	// CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))
	CreateEmptyDatabase_TestHelper( t, SqliteDbInMemory)

	helloFile_db_path := "arbitrary / file path / hello,pdf"
	sha_hello := RecordPdf_TestHelper( t, pdfPath_hello, helloFile_db_path)

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
			PictureRoot:     jpgRoot,
			EnableCORS:      true,
			MaxUploadSize:   10240,
			ShutdownTimeout: 5000,
		}
		ready := make(chan struct{})
		server := Server{config,}
		go func() {
			t.Logf("starting server at %s", target)
			server.Start(ctx, ready) // nolint:errcheck
		}()
		<-ready
	}

	base, err := url.Parse("http://" + target)
	require.Nil( t, err, fmt.Sprintf( "failed to parse base url: %v", err))

	var lastPdfSha string

	t.Run("GET /list", func(t *testing.T) {
		u := base.JoinPath("/list")
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusNotFound)
		expected := SuccessFileInfosResult{ false, nil,}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("PUT /file?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		GenerateUrl_TestHelper( t, u, pdfFilePath)
		req, sha := makeFormRequest( t, u, http.MethodPut, []string{ "lorem ipsum",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusCreated)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, sha},},}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join( jpgRoot, "hello-01.jpg"))
		lastPdfSha = sha
	})

	t.Run("GET /list", func(t *testing.T) {
		u := base.JoinPath("/list")
		pdfFilePath := "hello.pdf"
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, lastPdfSha},},}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("POST /file?name=foo.pdf should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "foo.pdf"
		GenerateUrl_TestHelper( t, u, pdfFilePath)
		req, _ := makeFormRequest( t, u, http.MethodPut, []string{ "hello, world",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to POST: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		var result ErrorResult
		err6 := json.Unmarshal(body, &result)
		assert.Nil(t, err6, fmt.Sprintf("failed to decode response body: %v", err6))
		expected := ErrorResult{false, "POST is not allowed on /file"}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})
/*
	t.Run("POST /upload should fail due to duplication", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "test.txt")
		localOriginal, err := os.ReadFile( localPath)
		if err != nil {
			t.Fatalf("failed to read local file: %v", err)
		}

		u := base.JoinPath("/upload")
		req, sha := makeFormRequest( t, u, http.MethodPut, []string{ "hello, new world",}, pdfRoot)
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
		verifyLocalFile(t, localPath, localOriginal)
	})
*/
	t.Run("PUT /file?name=arbitrary / file / path/hello.pdf should succeed even on overwrite", func(t *testing.T) {
		pdfFilePath := "hello.pdf"

		CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfFilePath)
		file, err1 := os.Open( pdfFilePath)
		assert.Nil(t, err1)
		defer file.Close()
		shaOld, err2 := calcSha( pdfFilePath)
		assert.Nil(t, err2)

		helloFile_db_dir := "arbitrary / file "
		helloFile_db_path := helloFile_db_dir + "/" + "hello,pdf"
		sha_hello := RecordFile_TestHelper( t, pdfFilePath, helloFile_db_dir, helloFile_db_path, 2, 1)
		assert.Equal( t, shaOld, sha_hello)

		u := base.JoinPath("/file")
		GenerateUrl_TestHelper( t, u, helloFile_db_path)
		req, shaNew := makeFormRequest( t, u, http.MethodPut, []string{ "hello, new world", "with second side"}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusCreated)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ helloFile_db_path, shaNew},},}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join(jpgRoot, "hello-01.jpg"))
	})
/*
	t.Run("PUT /files/hello_put.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest( t, u, http.MethodPut, "hello.txt", content)
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
		verifyLocalFile(t, filepath.Join(docRoot, "hello_put.txt"), []byte("hello, world"))
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
		info, err := os.Stat(filepath.Join(docRoot, "hello"))
		exists := !os.IsNotExist(err) && info.IsDir()
		if  err != nil {
			t.Fatalf("failed to check if directory exists: %v", err)
		} else if !exists {
			t.Errorf("directory /hello does not exist")
		}
		verifyLocalFile(t, filepath.Join(docRoot, "hello", "world.txt"), []byte("hello, world"))
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
			verifyLocalFile(t, filepath.Join(docRoot, "foo", "bar.txt"), []byte("new world"))
		})
	})
*/
	t.Run("GET /file?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, pdfFilePath)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, lastPdfSha},},}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("GET /files?name=foo/bar/baz.txt should fail (not found)", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "foo/bar/baz.txt"
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, pdfFilePath)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusNotFound)
		expected := SuccessFileInfosResult{ false, nil,}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("DELETE /file?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		req := GenerateRequest_TestHelper( t, http.MethodDelete, u, pdfFilePath)
		resp, err2 := http.DefaultClient.Do(req)
		assert.Nil(t, err2, fmt.Sprintf("failed to DELETE: %v", err2))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ pdfFilePath, lastPdfSha},},}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("GET /file?name=hello.pdf should fail now", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "hello.pdf"
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, pdfFilePath)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusNotFound)
		expected := SuccessFileInfosResult{ false, nil,}
		assert.Equal(t, expected.OK, result.OK, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.Equal(t, expected.Files, result.Files, fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, slices.Equal( expected.Files, result.Files), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("OPTIONS /list", func(t *testing.T) {
		u := base.JoinPath("/list")
		req, err1 := http.NewRequest(http.MethodOptions, u.String(), nil)
		assert.Nil(t, err1)
		resp, err2 := http.DefaultClient.Do(req)
		assert.Nil(t, err2)
		defer resp.Body.Close()
		CheckResponseCode_TestHelper( t, resp, http.StatusNoContent)
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
		CheckResponseCode_TestHelper( t, resp, http.StatusNoContent)
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

	if err := os.MkdirAll(pdfRoot, 0755); err != nil {
		t.Fatalf("failed to create document root: %v", err)
	}
	_ := CreatePdfFile_TestHelper( t, []string{ "lorem ipsum",}, path.Join(pdfRoot, "test.pdf"))
	_ := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, path.Join(pdfRoot, "foo", "bar.pdf"))

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
	server := Server{config,}
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
		verifyLocalFile(t, filepath.Join(docRoot, "hello.txt"), []byte("hello, world"))
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
		verifyLocalFile(t, filepath.Join(docRoot, "hello_query.txt"), []byte("hello, world"))
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
		verifyLocalFile(t, filepath.Join(docRoot, "hello_put.txt"), []byte("hello, world"))
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
func verifyLocalFile(t *testing.T, path string, content []byte) {
	got, err := os.ReadFile( path)
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
func makeFormRequest( t *testing.T, url *url.URL, method string, contentPages []string, pdfRoot string) (*http.Request, string) {
	filename := CreatePdfFile_TestHelper( t, contentPages, pdfRoot)
	sha, err0 := calcSha( filename)
	require.Nil(t, err0)
	file, err1 := os.Open( filename)
	require.Nil(t, err1)
	defer file.Close()
	b := new(bytes.Buffer)
	w := multipart.NewWriter(b)
	defer w.Close()
	fw, err2 := w.CreateFormFile("file", filename)
	require.Nil(t, err2)
	defer fw.Close()
	written, err3 := io.Copy(fw, file)
	assert.Nil(t, err3)
	assert.NotEqual(t, 0, written, fmt.Sprintf("no bytes written"))
	w.Close()
	req, err4 := http.NewRequest( method, url.String(), b)
	require.Nil(t, err4)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, sha
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
