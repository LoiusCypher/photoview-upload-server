package uploadserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/mux"
	"github.com/spf13/afero"
)

type Server struct {
	ServerConfig
	fs afero.Fs
}

var (
	FormFileKey       = "file"
	OverwriteQueryKey = "overwrite"
)

var (
	ErrFileSizeLimitExceeded = fmt.Errorf("file size limit exceeded")
)

var (
	DefaultAddr = "127.0.0.1:8080"
)

// ServerConfig is a configuration for Server.
type ServerConfig struct {
	// Address where the server listens on.
	Addr string `json:"addr"`
	// Path to the document root.
	DocumentRoot string `json:"document_root"`
	// Determines whether to enable CORS header.
	EnableCORS bool `json:"enable_cors"`
	// Maximum upload size in bytes.
	MaxUploadSize int64 `json:"max_upload_size"`
	// File naming strategy.
	FileNamingStrategy string `json:"file_naming_strategy"`
	// Graceful shutdown timeout in milliseconds.
	ShutdownTimeout int `json:"shutdown_timeout"`
	// Enable authentication.
	EnableAuth bool `json:"enable_auth"`
	// Authentication tokens for read-only access.
	ReadOnlyTokens []string `json:"read_only_tokens"`
	// Authentication tokens for read-write access.
	ReadWriteTokens []string `json:"read_write_tokens"`
	// ReadTimeout is the maximum duration (in seconds) for reading the entire request, including the body. Zero or negative value means no timeout.
	ReadTimeout time.Duration `json:"read_timeout"`
	// WriteTimeout is the maximum duration (in seconds) for writing the response. Zero or negative value means no timeout.
	WriteTimeout time.Duration `json:"write_timeout"`
}

// NewServer creates a new Server.
func NewServer(config ServerConfig) *Server {
	return &Server{
		config,
		afero.NewBasePathFs(afero.NewOsFs(), config.DocumentRoot),
	}
}

// Start starts listening on `addr`. This function blocks until the server is stopped.
// Optionally you can pass a channel to `ready` to be notified when the server is ready to accept connections. You can pass nil if you don't need it.
func (s *Server) Start(ctx context.Context, ready chan struct{}) error {
	r := mux.NewRouter()
	r.PathPrefix("/filelist").Methods(http.MethodGet).HandlerFunc(s.handle(s.handleListGet))
	r.PathPrefix("/filelist").Methods(http.MethodOptions).HandlerFunc(s.handle(s.handleOptions))
	r.PathPrefix("/files").Methods(http.MethodGet).HandlerFunc(s.handle(s.handleGet))
	r.PathPrefix("/files").Methods(http.MethodPut).HandlerFunc(s.handle(s.handlePut))
	r.PathPrefix("/files").Methods(http.MethodDelete).HandlerFunc(s.handle(s.handleDelete))
	r.PathPrefix("/files").Methods(http.MethodOptions).HandlerFunc(s.handle(s.handleOptions))
	r.NotFoundHandler = http.HandlerFunc(handleNotFound)
	r.MethodNotAllowedHandler = http.HandlerFunc(handleMethodNotAllowed)
	if s.EnableAuth {
		r.Use(s.authenticationMiddleware)
	}
	r.Use(logAccess)

	addr := s.Addr
	if addr == "" {
		addr = DefaultAddr
	}
	log.Printf("Start listening on %s", addr)
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("unable to listen on %s: %v", addr, err)
	}
	if ready != nil {
		close(ready)
	}

	srv := &http.Server{
		Addr:         addr,
		WriteTimeout: s.WriteTimeout,
		ReadTimeout:  s.ReadTimeout,
		IdleTimeout:  60 * time.Second,
		Handler:      r,
	}

	ret := make(chan error, 1)
	go func() {
		log.Printf("Start serving on %s", addr)
		ret <- srv.Serve(l)
	}()

	<-ctx.Done()
	log.Printf("Shutting down... wait up to %d ms", s.ShutdownTimeout)
	sctx, cancel := context.WithTimeout(context.Background(), time.Duration(s.ShutdownTimeout)*time.Millisecond)
	defer cancel()
	if err := srv.Shutdown(sctx); err != nil {
		log.Printf("failed to shutdown gracefully: %v", err)
	}
	err = <-ret
	return err
}

func logAccess(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		vs := []string{
			r.RemoteAddr,
			"-",
			"-",
			time.Now().Format("[02/Jan/2006:15:04:05 -0700]"),
			fmt.Sprintf("\"%s %s %s\"", r.Method, r.URL.Path, r.Proto),
			fmt.Sprintf("%d", http.StatusOK), // TODO: actual status
			"0",                              // TODO: actual size
			fmt.Sprintf("\"%s\"", r.Referer()),
			fmt.Sprintf("\"%s\"", r.UserAgent()),
		}
		log.Println(strings.Join(vs, " "))
		next.ServeHTTP(w, r)
	})
}

type ErrorResult struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

type SuccessfullyUploadedResult struct {
	OK   bool   `json:"ok"`
	Path string `json:"path"`
}

func justOK() (int, any) {
	return 0, nil
}

func (s *Server) handle(f func(w http.ResponseWriter, r *http.Request) (int, any)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status, result := f(w, r)
		var responseBody []byte
		if result != nil {
			switch v := result.(type) {
			case error:
				result = ErrorResult{false, v.Error()}
			}
			respBytes, err := json.Marshal(result)
			if err != nil {
				log.Printf("failed to encode response: %v", err)
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			responseBody = respBytes
		}
		if responseBody != nil {
			w.Header().Set("Content-Type", "application/json")
			if status != 0 {
				w.WriteHeader(status)
			}
			if _, err := w.Write(responseBody); err != nil {
				log.Printf("failed to write response: %v", err)
			}
		} else {
			if status != 0 {
				w.WriteHeader(status)
			}
		}
	}
}

func (s *Server) handlePut(w http.ResponseWriter, r *http.Request) (int, any) {
	path := strings.TrimPrefix(r.URL.Path, "/files/")

	status, pdfPath, err := s.processUpload(w, r, path)
	if err != nil {
		return status, err
	}
	err = convertPdfToJpg( pdfPath, s.DocumentRoot, path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	err = recordSha( pdfPath, path)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	err = os.Remove(pdfPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return http.StatusCreated, SuccessfullyUploadedResult{true, r.URL.Path}
}

func (s *Server) handleDelete(w http.ResponseWriter, r *http.Request) (int, any) {
	path := strings.TrimPrefix(r.URL.Path, "/files/")

	err := deleteJpgs( s.DocumentRoot, path)
	if err != nil {
		return http.StatusNotFound, err
	}
	err = deleteSha( path)
	if err != nil {
		return http.StatusNotFound, err
	}

	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return justOK()
}

func (s *Server) processUpload(w http.ResponseWriter, r *http.Request, path string) (int, string, error) {
	srcFile, _, err := r.FormFile(FormFileKey)
	if err != nil {
		log.Printf("failed to obtain form file: %v", err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot obtain the uploaded content")
	}
	src := http.MaxBytesReader(w, srcFile, s.MaxUploadSize)
	// MaxBytesReader closes the underlying io.Reader on its Close() is called
	defer src.Close()

	dstFile, err := ioutil.TempFile( "", "*.pdf")
	if err != nil {
		log.Printf("failed to create temp file (path=%s): %v", path, err)
		return http.StatusInternalServerError, "", fmt.Errorf("cannot open file")
	}
	defer os.Remove(dstFile.Name())
	written, err := io.Copy(dstFile, src)
	if err != nil {
		var maxBytesError *http.MaxBytesError
		if errors.As(err, &maxBytesError) {
			return http.StatusRequestEntityTooLarge, "", ErrFileSizeLimitExceeded
		}
		log.Printf("failed to write the uploaded content: %v", err)
		return http.StatusInternalServerError, "", fmt.Errorf("failed to write the content")
	}
	log.Printf("uploaded by PUT to %s (%d bytes)", dstFile.Name(), written)
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	return http.StatusCreated, dstFile.Name(), nil
}

func (s *Server) handleListGet(w http.ResponseWriter, r *http.Request) (int, any) {
	requestPath := strings.TrimPrefix(r.URL.Path, "/filelist")
	requestPath = strings.TrimPrefix(requestPath, "/")
	log.Printf("GET %s -> %s", r.URL.Path, requestPath)
	found, err := ListShas(w, requestPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Add("Last-Modified", time.Now().String())
	if !found {
		return http.StatusNotFound, "file not Found"
	}
	return justOK()
}

func (s *Server) handleGet(w http.ResponseWriter, r *http.Request) (int, any) {
	requestPath := strings.TrimPrefix(r.URL.Path, "/files/")
	log.Printf("GET %s -> %s", r.URL.Path, requestPath)
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return http.StatusInternalServerError, fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	found, err := shaDb.getPathSha(w, requestPath)
	if err != nil {
		return http.StatusInternalServerError, err
	}
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Add("Last-Modified", time.Now().String())
	if !found {
		return http.StatusNotFound, "file not Found"
	}
	return justOK()
}

func (s *Server) authenticationMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// OPTIONS request is always allowed without authentication
		if r.Method == http.MethodOptions {
			next.ServeHTTP(w, r)
			return
		}

		var token string
		if auth := r.Header.Get("Authorization"); auth != "" {
			token = strings.TrimPrefix(auth, "Bearer ")
		} else if t := r.URL.Query().Get("token"); t != "" {
			token = t
		}
		if token == "" {
			log.Printf("no token")
			writeUnauthorized(w, r)
			return
		}
		var allowedTokens []string
		allowedTokens = append(allowedTokens, s.ReadWriteTokens...)
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			allowedTokens = append(allowedTokens, s.ReadOnlyTokens...)
		}
		if !slices.Contains(allowedTokens, token) {
			log.Printf("invalid token")
			writeUnauthorized(w, r)
			return
		}
		log.Print("successfully authenticated")
		r.Header.Del("Authorization")
		u := r.URL
		q := u.Query()
		q.Del("token")
		u.RawQuery = q.Encode()
		r.URL = u
		next.ServeHTTP(w, r)
	})
}

func writeUnauthorized(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("WWW-Authenticate", "Bearer")
	if r.Method != http.MethodHead {
		w.Header().Set("Content-Type", "application/json")
	}
	w.WriteHeader(http.StatusUnauthorized)
	if r.Method == http.MethodHead {
		return
	}
	resp := ErrorResult{false, "unauthorized"}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func handleNotFound(w http.ResponseWriter, r *http.Request) {
	resp := ErrorResult{false, "not found"}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

var allowedMethodsFilelist = []string{http.MethodGet}
var allowedMethodsFiles = []string{http.MethodGet, http.MethodPut, http.MethodDelete}

func (s *Server) handleOptions(w http.ResponseWriter, r *http.Request) (int, any) {
	var allowedMethods []string
	if r.URL.Path == "/filelist" {
		allowedMethods = allowedMethodsFilelist
	} else if strings.HasPrefix(r.URL.Path, "/files") {
		allowedMethods = allowedMethodsFiles
	}
	if s.EnableCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
	}
	w.Header().Set("Access-Control-Allow-Methods", strings.Join(allowedMethods, ", "))
	return http.StatusNoContent, nil
}

func handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	var endpoint string
	var allowedMethods []string
	if r.URL.Path == "/filelist" {
		endpoint = "/filelist"
		allowedMethods = allowedMethodsFilelist
	}
	if strings.HasPrefix(r.URL.Path, "/files") {
		endpoint = "/files"
		allowedMethods = allowedMethodsFiles
	}
	w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	resp := ErrorResult{false, fmt.Sprintf("%s is not allowed on %s", r.Method, endpoint)}
	respBytes, err := json.Marshal(resp)
	if err != nil {
		log.Printf("failed to encode response: %v", err)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusMethodNotAllowed)
	if _, err := w.Write(respBytes); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

