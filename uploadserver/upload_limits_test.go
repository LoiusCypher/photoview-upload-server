package uploadserver

import (
	"bytes"
	"io"
	"math"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

type uploadBodyCounter struct {
	io.ReadCloser
	read int64
}

func (b *uploadBodyCounter) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.read += int64(n)
	return n, err
}

func uploadLimitRequest(t *testing.T, method, urlPath, dstPathFormValue, uploadFilePath, field string, size, extra int) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	file, err := mw.CreateFormFile(field, uploadFilePath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.CopyN(file, endlessZeros{}, int64(size)); err != nil {
		t.Fatal(err)
	}
	if dstPathFormValue != "" {
		err := mw.WriteField( "name", dstPathFormValue)
		if err != nil {
			t.Fatal(err)
		}
	}
	if extra > 0 {
		field, err := mw.CreateFormField("extra")
		if err != nil {
			t.Fatal(err)
		}
		if _, err := io.CopyN(field, endlessZeros{}, int64(extra)); err != nil {
			t.Fatal(err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(method, urlPath, &body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

type endlessZeros struct{}

func (endlessZeros) Read(p []byte) (int, error) {
	clear(p)
	return len(p), nil
}

func uploadLimitRouter(s *Server) http.Handler {
	r := http.NewServeMux()
	r.HandleFunc( "POST /upload/{$}", s.handle(s.handlePostUpload))
	return r

}

func TestUploadLimits(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

	// ghostscript output has to be on real file system
	pictureRoot, err1 := os.MkdirTemp( "", "")
	require.Nil(t, err1)
	defer os.RemoveAll( pictureRoot)

	srcRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( srcRoot)

	for _, method := range []string{http.MethodPost} {
		for _, unknownLength := range []bool{false, true} {
			for _, tc := range []struct {
				name                string
				limit               int64
				size, extra         int
				existing            bool
				want                int
			}{
				{name: "empty", limit: 16, want: 201},
				{name: "exact file limit", limit: 16, size: 16, want: 201},
				{name: "one byte over", limit: 16, size: 17, want: 413},
				{name: "oversized overwrite preserves original", limit: 16, size: 17, existing: true, want: 413},
				{name: "oversized conflict", limit: 16, size: 17, existing: true, want: 413},
				{name: "aggregate multipart limit", limit: 16, size: 1, extra: 2 << 20, want: 413},
				{name: "large selected part", limit: 16, size: 2 << 20, want: 413},
				{name: "conflict", limit: 16, size: 16, existing: true, want: 409},
				{name: "overwrite", limit: 16, size: 16, existing: true, want: 201},
				{name: "zero limit empty", want: 201},
				{name: "zero limit nonempty", size: 1, want: 413},
				{name: "negative limit empty", limit: -1, want: 201},
				{name: "negative limit nonempty", limit: -1, size: 1, want: 413},
				{name: "maximum integer limit", limit: math.MaxInt64, size: 16, want: 201},
			} {
				t.Run(method+"/"+map[bool]string{true: "unknown", false: "known"}[unknownLength]+"/"+tc.name, func(t *testing.T) {
					s := &Server{ServerConfig: ServerConfig{MaxUploadSize: tc.limit, PictureRoot: pictureRoot, },}
					var shaExisting string
					if tc.existing {
						if err := os.WriteFile( path.Join( srcRoot, "/limit.bin"), []byte("original"), 0600); err != nil {
							t.Fatal(err)
						}
						defer os.Remove( path.Join( srcRoot, "limit.bin"))
						var err error
						shaExisting, err = calcSha( path.Join( srcRoot, "limit.bin"))
						if  err != nil {
							t.Fatal(err)
						}
						shaExisting, err = recordFileSha( shaExisting, "limit.bin")
						if  err != nil || shaExisting == "" {
							t.Fatal(err)
						}
					}
					urlPath := "/upload/?name=limit.bin"
					handler := uploadLimitRouter(s)
					attempts := 1
					if tc.want == 413 {
						attempts = 2
					}
					for attempt := 0; attempt < attempts; attempt++ {
						req := uploadLimitRequest(t, method, urlPath, "", path.Join( srcRoot, "/limit.bin"), "file", tc.size, tc.extra)
						if unknownLength {
							req.ContentLength = -1
						}
						counter := &uploadBodyCounter{ReadCloser: req.Body}
						req.Body = counter
						rr := httptest.NewRecorder()
						handler.ServeHTTP(rr, req)
						if rr.Code != tc.want {
							t.Fatalf("status=%d, want=%d: %s", rr.Code, tc.want, rr.Body.String())
						}
						if tc.want == 413 && rr.Body.String() != `{"ok":false,"error":"file size limit exceeded"}` {
							t.Fatalf("unexpected body: %s", rr.Body.String())
						}
						if tc.limit >= 0 && tc.limit < math.MaxInt64-(1<<20)-1 && counter.read > tc.limit+(1<<20)+1 {
							t.Fatalf("read %d bytes beyond body cap", counter.read)
						}
					}
					// We should never leave files in upload
					entries, err1 := filepath.Glob( filepath.Join( root, "*"))
					if err1 != nil {
						t.Fatal(err1)
					}
					if len(entries) > 0 {
						t.Fatal("rejected upload left files in upload:", len(entries), entries)
					}
					if tc.want == 413 || tc.want == 409 {
						currentSha, err := getPathSha( "limit.bin")
						if tc.existing {
							if err != nil || currentSha != shaExisting {
								t.Fatalf("existing file changed: %q, %q, %v", shaExisting, currentSha, err)
							}
						}
					}
				})
			}
		}
	}
}

func TestUploadMultipartTemporaryCleanup(t *testing.T) {
	// sqlite database has to be on real file system or in memory
	sqlRoot, err1 := os.MkdirTemp("", "")
	require.Nil( t, err1)
	defer os.RemoveAll( sqlRoot)
	CreateEmptyDatabase_TestHelper( t, path.Join( sqlRoot, "shafiles.db"))

		pictureRoot, err2 := os.MkdirTemp("", "")
		require.Nil(t, err2)
	defer os.RemoveAll( pictureRoot)

	originRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( originRoot)

	tmp := t.TempDir()
	t.Setenv("TMPDIR", tmp)
	t.Setenv("TMP", tmp)
	t.Setenv("TEMP", tmp)
	for _, method := range []string{http.MethodPost} {
		for _, tc := range []struct {
			name, field string
			limit       int64
			existing    bool
			want        int
		}{
			{name: "success", field: "file", limit: 34 << 20, want: 201},
			{name: "conflict", field: "file", limit: 34 << 20, existing: true, want: 409},
			{name: "missing selected file", field: "other", limit: 34 << 20, want: 500},
			{name: "file over limit after spill", field: "file", limit: (33 << 20) - 1, want: 413},
			{name: "body limit during spill", field: "file", limit: 32 << 20, want: 413},
		} {
			t.Run(method+"/"+tc.name, func(t *testing.T) {
				root := t.TempDir()
				s := NewServer(ServerConfig{PictureRoot: pictureRoot, UploadRoot: root, MaxUploadSize: tc.limit})
				var shaExisting string
				if tc.existing {
					if err := os.WriteFile( path.Join( root, "limit.bin"), []byte("original"), 0600); err != nil {
						t.Fatal(err)
					}
					defer os.Remove( path.Join( root, "limit.bin"))
					var err error
					shaExisting, err = calcSha( path.Join( root, "limit.bin"))
					if  err != nil {
						t.Fatal(err)
					}
					_, err = recordFileSha( shaExisting, "limit.bin")
					if  err != nil {
						t.Fatal(err)
					}
				}
				urlPath := "/upload/"
				req := uploadLimitRequest(t, method, urlPath, "limit.bin", path.Join( originRoot, "limit.bin"), tc.field, 33<<20, 0)
				rr := httptest.NewRecorder()
				uploadLimitRouter(s).ServeHTTP(rr, req)
				if rr.Code != tc.want {
					t.Fatalf("status=%d, want=%d: %s", rr.Code, tc.want, rr.Body.String())
				}
				// We should never leave files in upload
				entries, err1 := filepath.Glob( filepath.Join( root, "*"))
				if err1 != nil {
					t.Fatal(err1)
				}
				if len(entries) > 0 {
					t.Fatal("rejected upload left files in upload:", len(entries), entries)
				}
				if tc.want == 201 {
					currentSha, err := getPathSha( "limit.bin")
					if err != nil {
						t.Fatal(err)
					}
					if currentSha != "" {
						t.Fatalf("upload was not registered in database")
					}
				}
			})
		}
	}
}
