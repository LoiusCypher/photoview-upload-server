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
	"log"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)


func GenerateUrl_TestHelper(t *testing.T, u *url.URL, filePath string) {
	q := url.Values{}
        q.Add("name", filePath)
	u.RawQuery = q.Encode()
}

func GenerateRequest_TestHelper(t *testing.T, method string, u *url.URL, query string, filePath string) *http.Request {
	if query != "" {
		q := url.Values{}
        	q.Add( query, filePath)
		u.RawQuery = q.Encode()
	}
	log.Println( "Method:", method, "u:", u.String())
	req, err1 := http.NewRequest( method, u.String(), nil)
	assert.Nil(t, err1)
	return req
}

func CheckResponseCode_TestHelper(t *testing.T, resp *http.Response, statusCode int) {
	assert.Equal(t, statusCode, resp.StatusCode)
	if statusCode == http.StatusMethodNotAllowed {
		assert.Equal(t, "text/plain; charset=utf-8", resp.Header.Get("Content-Type"))
	} else {
		assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
	}
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
	require.Nil(t, err1)
	defer os.RemoveAll( jpgRoot)

	pdfRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( pdfRoot)
	_ = CreatePdfFile_TestHelper( t, []string{ "lorem ipsum",}, pdfRoot)
	pdfPath_hello := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfRoot)

	// sqlite database has to be on real file system or in memory
	sqlRoot, err3 := os.MkdirTemp("", "")
	require.Nil(t, err3)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	helloFile_db_path := "arbitrary / file path / hello.pdf"
	helloSha := RecordFile_TestHelper( t, pdfPath_hello, helloFile_db_path)
	fmt.Println("SHA 1", helloSha)
/*
	helloFile2_db_path := "arbitrary / file path / hello2.pdf"
	hello2Sha := RecordFile_TestHelper( t, pdfPath_hello, helloFile2_db_path)
	fmt.Println("SHA 2", hello2Sha)
*/
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

	type args struct {
		method  string
		url     string
		query   string
		value   string
		pages   []string
	}
	tests := []struct {
		name    string
		args    args
		want    int
		body    string
		headers map[string]string
	}{
		{
			name: " 1 expect one file from setup",
			args: args{
				method: http.MethodGet,
				url:    "/list/",
				query:  "",
				value:  "",
				pages:  nil,
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"` + helloFile_db_path + `","sha":"` + helloSha + `"}]}`,
		},
		{
			name: "?name",
			args: args{
				method: http.MethodGet,
				url:    "/list/",
				query:  "name",
				value:  "",
				pages:  nil,
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"` + helloFile_db_path + `","sha":"` + helloSha + `"}]}`,
		},
		{
			name: "?name",
			args: args{
				method: http.MethodGet,
				url:    "/list/all",
				query:  "name",
				value:  "",
				pages:  nil,
			},
			want: http.StatusMethodNotAllowed,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: "Method Not Allowed\nok",
		},
		{
			name: "?name",
			args: args{
				method: http.MethodPost,
				url:    "/upload/",
				query:  "",
				value:  "",
				pages:  []string{ "lorem ipsum",},
			},
			want: http.StatusOK,
			headers: map[string]string{
				"Access-Control-Allow-Origin": "*",
			},
			body: `{"ok":true,"Files":[{"path":"` + helloFile_db_path + `","sha":"` + helloSha + `"}]}`,
		},
	}
	for _, tt := range tests {
		t.Run( tt.args.method + " " + tt.args.url + tt.name, func(t *testing.T) {
			u := base.JoinPath( tt.args.url)
			req, _ := GenerateFullRequest_TestHelper( t, tt.args.method, u, tt.args.query, tt.args.value, tt.args.pages, pdfRoot)
			resp, err4 := http.DefaultClient.Do(req)
			assert.Nil(t, err4, fmt.Sprintf("failed to %s: %v", tt.args.method, err4))
			defer resp.Body.Close()
			result := CheckResponseSuccess_TestHelper( t, resp, tt.want) 
			assert.Equal(t, fmt.Sprintf("%+v", result), tt.body, fmt.Sprintf("result = %+v, want = %+v", result, tt.body))
		})
	}

	t.Run("GET /list/ 3 expect one file from setup", func(t *testing.T) {
		u := base.JoinPath("/list/")
log.Println( "u:", u.String())
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ helloFile_db_path, helloSha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("GET /list/?name=", func(t *testing.T) {
		u := base.JoinPath("/list/")
		GenerateUrl_TestHelper( t, u, "")
log.Println( "u:", u.String())
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ helloFile_db_path, helloSha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("GET /list/all?name=", func(t *testing.T) {
		u := base.JoinPath("/list/all")
		GenerateUrl_TestHelper( t, u, "")
log.Println( "u:", u.String())
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		assert.Equal(t, string(body), "Method Not Allowed\n")
	})

	t.Run("POST /upload/ hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/upload/")
		pdfFilePath := "hello.pdf"
		req, sha := makeFormRequest( t, u, http.MethodPost, pdfFilePath, []string{ "lorem ipsum",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to POST: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusCreated)
		expected := SuccessFileInfosResult{ true, []FileInfo{ { pdfFilePath, sha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join( jpgRoot, "hello", "hello-001.jpg"))
		lastPdfSha = sha
	})

	t.Run("POST /upload/ hello.pdf overwrite should work the same", func(t *testing.T) {
		u := base.JoinPath("/upload/")
		pdfFilePath := "hello.pdf"
		req, sha := makeFormRequest( t, u, http.MethodPost, pdfFilePath, []string{ "new world", "page two",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to POST: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusCreated)
		expected := SuccessFileInfosResult{ true, []FileInfo{ { pdfFilePath, sha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join( jpgRoot, "hello", "hello-001.jpg"))
		lastPdfSha = sha
	})

	t.Run("GET /list expect sha changed from update", func(t *testing.T) {
		u := base.JoinPath("/list")
log.Println( "u:", u.String())
		pdfFilePath := "hello.pdf"
		resp, err4 := http.Get(u.String())
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusOK)
		expected := SuccessFileInfosResult{ true, []FileInfo{ { helloFile_db_path, helloSha}, { pdfFilePath, lastPdfSha},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
	})

	t.Run("PUT /list/ should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/list/")
		pdfFilePath := "foo.pdf"
		req, _ := makeFormRequest( t, u, http.MethodPut, pdfFilePath, []string{ "hello, world",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		assert.Equal(t, string(body), "Method Not Allowed\n")
	})

	t.Run("PUT /file/ foo.pdf should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/file/")
		pdfFilePath := "foo.pdf"
		req, _ := makeFormRequest( t, u, http.MethodPut, pdfFilePath, []string{ "hello, world",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		assert.Equal(t, string(body), "Method Not Allowed\n")
	})

	t.Run("PUT /upload/ foo.pdf should fail due to an invalid method", func(t *testing.T) {
		u := base.JoinPath("/upload/")
		pdfFilePath := "foo.pdf"
		req, _ := makeFormRequest( t, u, http.MethodPut, pdfFilePath, []string{ "hello, world",}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to PUT: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		assert.Equal(t, string(body), "Method Not Allowed\n")
	})

	t.Run("POST /upload arbitrary / file / path/hello.pdf should succeed even on overwrite", func(t *testing.T) {
		pdfFilePath := CreatePdfFile_TestHelper( t, []string{ "hello, world",}, pdfRoot)
		sha_file, err2 := calcSha( pdfFilePath)
		assert.Nil(t, err2)
		fmt.Println("SHA 2", sha_file)

		helloFile_db_dir := "arbitrary / file "
		helloFile_db_path := helloFile_db_dir + "/" + "hello.pdf"
		sha_hello := RecordPdfSha_TestHelper( t, sha_file, helloFile_db_path)
		assert.Equal( t, sha_file, sha_hello)

		u := base.JoinPath("/upload")
		//GenerateUrl_TestHelper( t, u, helloFile_db_path)
		req, shaNew := makeFormRequest( t, u, http.MethodPost, helloFile_db_path, []string{ "hello, new world", "with second side"}, pdfRoot)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to POST: %v", err4))
		defer resp.Body.Close()
		result := CheckResponseSuccess_TestHelper( t, resp, http.StatusCreated)
		expected := SuccessFileInfosResult{ true, []FileInfo{{ helloFile_db_path, shaNew},},}
		assert.True(t, reflect.DeepEqual(result, expected), fmt.Sprintf("result = %+v, want = %+v", result, expected))
		assert.FileExists(t, filepath.Join(jpgRoot, "hello", "hello-001.jpg"))
	})
/*
	t.Run("POST /upload/hello_put.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest( t, u, http.MethodPost, "hello.txt", content)
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
	t.Run("POST /upload/hello/world.txt", func(t *testing.T) {
		u := base.JoinPath("/files/hello/world.txt")
		req, err := makeFormRequest(u, http.MethodPost, "world.txt", bytes.NewBufferString("hello, world"))
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
	t.Run("POST /upload should fail", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "foo.txt", bytes.NewBufferString("hello, world"))
		if err != nil {
			t.Fatal(err)
		}
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("failed to POST: %v", err)
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
		expected := ErrorResult{false, "POST is not allowed on /upload"}
		if !reflect.DeepEqual(result, expected) {
			t.Errorf("result = %+v, want = %+v", result, expected)
		}
	})
*/ /*
	t.Run("POST /upload/foo/bar.txt should fail (conflict)", func(t *testing.T) {
		u := base.JoinPath("/files/foo/bar.txt")
		req, err := makeFormRequest(u, http.MethodPost, "foo.txt", bytes.NewBufferString("new world"))
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
	})
*/ /*
	t.Run("POST /upload/foo/bar.txt should succeed (overwrite)", func(t *testing.T) {
		localPath := filepath.Join(docRoot, "foo", "bar.txt")
		withPreservingOriginal(t, localPath, func() {
			u := base.JoinPath("/files/foo/bar.txt")
			q := u.Query()
			q.Set("overwrite", "true")
			u.RawQuery = q.Encode()

			req, err := makeFormRequest(u, http.MethodPost, "foo.txt", bytes.NewBufferString("new world"))
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
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, "name", pdfFilePath)
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

	t.Run("GET /file/all?name=hello.pdf", func(t *testing.T) {
		u := base.JoinPath("/file/all")
		pdfFilePath := "hello.pdf"
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, "name", pdfFilePath)
		resp, err4 := http.DefaultClient.Do(req)
		assert.Nil(t, err4, fmt.Sprintf("failed to GET: %v", err4))
		defer resp.Body.Close()
		body := CheckResponse_TestHelper( t, resp, http.StatusMethodNotAllowed)
		assert.Equal(t, string(body), "Method Not Allowed\n")
	})

	t.Run("GET /files?name=foo/bar/baz.txt should fail (not found)", func(t *testing.T) {
		u := base.JoinPath("/file")
		pdfFilePath := "foo/bar/baz.txt"
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, "name", pdfFilePath)
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
		req := GenerateRequest_TestHelper( t, http.MethodDelete, u, "name", pdfFilePath)
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
		req := GenerateRequest_TestHelper( t, http.MethodGet, u, "name", pdfFilePath)
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

	t.Run("OPTIONS /list/", func(t *testing.T) {
		u := base.JoinPath("/list/")
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

	t.Run("OPTIONS /file/", func(t *testing.T) {
		u := base.JoinPath("/file/")
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
		expectedAllowedMethods := []string{ "GET", "DELETE",}
		assert.True( t, containsAll(allowedMethods, expectedAllowedMethods), fmt.Sprintf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods))
		assert.Equal( t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
	})

	t.Run("OPTIONS /upload/", func(t *testing.T) {
		u := base.JoinPath("/upload/")
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
		expectedAllowedMethods := []string{ "POST",}
		assert.True( t, containsAll(allowedMethods, expectedAllowedMethods), fmt.Sprintf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods))
		assert.Equal( t, resp.Header.Get("Access-Control-Allow-Origin"), "*")
	})
}

func TestServerWithAuth(t *testing.T) {
	pdfRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( pdfRoot)

	// ghostscript output has to be on real file system
	jpgRoot, err1 := os.MkdirTemp( "", "")
	require.Nil(t, err1)
	defer os.RemoveAll( jpgRoot)

	if err := os.MkdirAll(pdfRoot, 0755); err != nil {
		t.Fatalf("failed to create document root: %v", err)
	}
	_ = CreatePdfFile_TestHelper( t, []string{ "lorem ipsum",}, path.Join(pdfRoot, "test.pdf"))
	_ = CreatePdfFile_TestHelper( t, []string{ "hello, world",}, path.Join(pdfRoot, "foo", "bar.pdf"))

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
		PictureRoot:     jpgRoot,
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
/*
	t.Run("POST /upload token via header", func(t *testing.T) {
		u := base.JoinPath("/upload")
		req, err := makeFormRequest(u, http.MethodPost, "hello.pdf", bytes.NewBufferString("hello, world"), pdfRoot)
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
*//*
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
*//*
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
*//*
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
*//*
	t.Run("POST /files/hello_put.txt with read-write token", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", content)
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
*//*
	t.Run("POST /upload/hello_put.txt with read-only token", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", content)
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
*//*
	t.Run("POST /upload/hello_put.txt without tokens", func(t *testing.T) {
		u := base.JoinPath("/files/hello_put.txt")
		content := bytes.NewBufferString("hello, world")
		req, err := makeFormRequest(u, http.MethodPost, "hello.txt", content)
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
*//*
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
*//*
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
*//*
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
*/
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
/*
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
*//*
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
*//*
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
*//*
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
*//*
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
		expectedAllowedMethods := []string{"GET", "HEAD", "POST"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})
*//*
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
		expectedAllowedMethods := []string{"GET", "HEAD", "POST"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})
*//*
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
		expectedAllowedMethods := []string{"GET", "HEAD", "POST"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})
*//*
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
		expectedAllowedMethods := []string{"GET", "HEAD", "POST"}
		if !containsAll(allowedMethods, expectedAllowedMethods) {
			t.Errorf("Access-Control-Allow-Methods = %v, want = %v", allowedMethods, expectedAllowedMethods)
		}
		if acao := resp.Header.Get("Access-Control-Allow-Origin"); acao != "*" {
			t.Errorf("Access-Control-Allow-Origin = %s, want = \"*\"", acao)
		}
	})
*/}
/*
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
func makeFormRequest( t *testing.T, url *url.URL, method string, filePath string, contentPages []string, pdfRoot string) (*http.Request, string) {
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
	// defer fw.Close()
	written, err3 := io.Copy(fw, file)
	assert.Nil(t, err3)
	// assert.NotEqual(t, 0, written, fmt.Sprintf("no bytes written"))
	assert.NotEqual(t, 0, written, "no bytes written")
	err4 := w.WriteField( "name", filePath)
	assert.Nil(t, err4)
	w.Close()
	req, err5 := http.NewRequest( method, url.String(), b)
	require.Nil(t, err5)
	req.Header.Set("Content-Type", w.FormDataContentType())
	return req, sha
}

func GenerateFullRequest_TestHelper(t *testing.T, method string, u *url.URL, query string, filePath string, contentPages []string, pdfRoot string) (*http.Request, string) {
	if query != "" {
		q := url.Values{}
        	q.Add( query, filePath)
		u.RawQuery = q.Encode()
	}
	log.Println( "Method:", method, "u:", u.String())
	if len( contentPages) > 0 {
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
		// defer fw.Close()
		written, err3 := io.Copy(fw, file)
		assert.Nil(t, err3)
		// assert.NotEqual(t, 0, written, fmt.Sprintf("no bytes written"))
		assert.NotEqual(t, 0, written, "no bytes written")
		err4 := w.WriteField( "name", filePath)
		assert.Nil(t, err4)
		w.Close()
		req, err5 := http.NewRequest( method, u.String(), b)
		require.Nil(t, err5)
		return req, sha
	} else {
		req, err1 := http.NewRequest( method, u.String(), nil)
		assert.Nil(t, err1)
		return req, ""
	}
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
