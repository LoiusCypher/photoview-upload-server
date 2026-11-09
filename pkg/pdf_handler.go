package simpleuploadserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	_ "github.com/mattn/go-sqlite3"
)

func calcSha(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file", filePath, ": ", err)
		return "", fmt.Errorf("Error opening file %s: %v", filePath, err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		log.Println("Error calculating file hash", filePath, ": ", err)
		return "", fmt.Errorf("Error calculating file hash %s: %v", filePath, err)
	}
	// log.Println("file hash", filePath, ": ", hex.EncodeToString(hasher.Sum(nil)))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func createJpgFromPdf(pathExe string, pdfPath string, jpgPath string) error {
	log.Printf("createJpgFromPdf: %s %s %s", pathExe, pdfPath, jpgPath)
	cmdArgs := []string{ "-dNOPAUSE", "-sDEVICE=jpeg", "-r100", "-sOutputFile=" + jpgPath, "-f", filepath.Base(pdfPath), "-c", "quit"}
	cmd := exec.Command(pathExe, cmdArgs...)
	cmd.Dir = filepath.Dir(pdfPath)
	var out strings.Builder
	cmd.Stdout = &out
	if err := cmd.Run(); err != nil {
		log.Println("cmd workdir:", filepath.Dir(pdfPath))
		log.Println("cmd:", pathExe, cmdArgs)
		log.Printf("stdout: %q\n", out.String())
        	switch {
			case errors.Is(err, exec.ErrNotFound):
				log.Printf("cmd.Run: command not found %v", err)
				return fmt.Errorf("command not found: %v", err)
			case errors.Is(err, os.ErrPermission):
				log.Printf("cmd.Run: permission denied %v", err)
				return fmt.Errorf("permission denied: %v", err)
			default:
				log.Printf("cmd.Run: execution error %v", err)
				return fmt.Errorf("execution error: %v", err)
		}
	}
	return nil
}

type ShaDatabase struct {
	sqlPath string
	db *sql.DB
}

func initDb (sqlPath string) error {
	// log.Printf("initDb: %s", sqlPath)
	// os.Remove("/sqlroot/shafiles.db")
	db, err := sql.Open("sqlite3", "/sqlroot/shafiles.db")
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer db.Close()
	sqlStmt := `CREATE TABLE IF NOT EXISTS shafiles (id INTEGER PRIMARY KEY AUTOINCREMENT, sha CHARACTER(64) NOT NULL, path TEXT UNIQUE NOT NULL);`
	_, err = db.Exec(sqlStmt)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return err
	}
	_, err = db.Exec(`CREATE INDEX IF NOT EXISTS shafiles_sha ON shafiles ( sha );`)
	if err != nil {
		log.Printf("%q: %s\n", err, sqlStmt)
		return err
	}
	return nil
}

func openDb (sqlPath string) *ShaDatabase {
	// log.Printf("openDb: %s", sqlPath)
	if err := initDb(sqlPath); err != nil {
		log.Fatal(err)
		return nil
	}

	db, err := sql.Open("sqlite3", sqlPath)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	shaDb := &ShaDatabase{ sqlPath, db, }
	return shaDb
}

func (shaDb *ShaDatabase) Close() {
	shaDb.db.Close()
	// log.Printf("CLOSE shaDb!!!!")
}

func (shaDb *ShaDatabase) countRows() error {
	log.Printf("countRows")
	var cnt int
	if err := shaDb.db.QueryRow("SELECT COUNT(*) FROM shafiles;").Scan(&cnt); err != nil {
		if err == sql.ErrNoRows {
			return nil
        	}
		return err
	}
	log.Printf("Count: %d", cnt)
	return nil
}

func (shaDb *ShaDatabase) getListShas(w http.ResponseWriter, rootDir string) ( bool, error) {
	log.Printf("getListShas: %s\n", rootDir)
	rows, err := shaDb.db.Query("SELECT sha, path FROM shafiles WHERE path LIKE ? ORDER BY path", rootDir + "%")
	if err != nil {
		log.Fatal(err)
		return false, err
	}
	defer rows.Close()
	for rows.Next() {
		var sha, path string
		err = rows.Scan(&sha, &path)
		if err != nil {
			log.Fatal(err)
			return false, err
		}
		// log.Printf("DB LIST %s: %s", sha, path)
		fmt.Fprintln(w, sha, " ", path)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
		return false, err
	}
	return true, nil
}

func (shaDb *ShaDatabase) getPathSha(w http.ResponseWriter, reqPath string) (bool, error) {
	// log.Printf("getPathSha: %s\n", reqPath)
	var sha string
	if err := shaDb.db.QueryRow("SELECT sha FROM shafiles WHERE path = ?;", reqPath).Scan(&sha); err != nil {
		if err == sql.ErrNoRows {
			return false, nil
        	}
		return false, err
	}
	fmt.Fprintln(w, sha)
	// log.Printf("getPathSha: %s\n", sha)
	return true, nil
}

func deleteJpgs(jpgroot string, uploadPath string) error {
	jpgPath := strings.TrimSuffix(jpgroot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-*.jpg"
	log.Printf("deleteJpgs: %s %s", uploadPath, jpgPath)
	jpgFiles, err := filepath.Glob(jpgPath)
	if err != nil {
		log.Println("Error resolving file pattern", jpgPath, ": ", err)
		return fmt.Errorf("Error resolving file pattern %s: %v", jpgPath, err)
	}
	for _, jpgFile := range jpgFiles {
		if err := os.Remove(jpgFile); err != nil {
			log.Println("Error removing", jpgFile, ": ", err)
			return fmt.Errorf("Error removing %s: %v", jpgFile, err)
		}
	}
	return nil
}

func convertPdfToJpg(pdfPath string, jpgRoot string, uploadPath string) error {
	var targExe string
	var pathExe string
	targExe = "gs"
	// targExe = "ls"
	pathExe, err := exec.LookPath(targExe)
	if err != nil {
		log.Printf("didn't find '%s' executable\n", targExe)
		return fmt.Errorf("didn't find %s: %v", targExe, err)
	}
	// log.Printf("'%s' executable magick in '%s'\n", targExe, pathExe)
	jpgPath := strings.TrimSuffix(jpgRoot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-%02d.jpg"
	err = os.MkdirAll(filepath.Dir(jpgPath), os.ModePerm)
	if err != nil {
		log.Printf("making jpgpath dir: %v\n", err)
		return fmt.Errorf("making jpgpath dir: %v", err)
	}
	log.Printf("convertPdfToJpg: %s %s %s", uploadPath, pdfPath, jpgPath)
	if err := createJpgFromPdf(pathExe, pdfPath, jpgPath); err != nil {
		return err
	}
	return nil
}

func recordSha(pdfPath string, registerPath string) error {
	pdfSha, err := calcSha(pdfPath)
	if err != nil {
		log.Printf("making calcSha: %v\n", err)
		return err
	}
	log.Println("file hash", pdfPath, ": ", pdfSha)
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	tx, err := shaDb.db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	_, err = tx.Exec("INSERT OR REPLACE INTO shafiles(sha, path) VALUES(?, ?)", pdfSha, registerPath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

func deleteSha(registerPath string) error {
	log.Printf("deleteSha: %s", registerPath)
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	tx, err := shaDb.db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	_, err = tx.Exec("DELETE FROM shafiles WHERE path = ?", registerPath)
	if err != nil {
		log.Fatal(err)
		return err
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
	}
	return nil
}

