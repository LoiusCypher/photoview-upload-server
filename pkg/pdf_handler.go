package simpleuploadserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"iter"
	"log"
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
	cmdArgs := []string{ "-dNOPAUSE", "-sDEVICE=jpeg", "-r300", "-sOutputFile=" + jpgPath, "-f", filepath.Base(pdfPath), "-c", "quit"}
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
	log.Printf("initDb: %s", sqlPath)
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
	log.Printf("openDb: %s", sqlPath)
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
	log.Printf("CLOSE shaDb!!!!")
}

func (shaDb *ShaDatabase) countRows() error {
	log.Printf("countRows")
	rows, err := shaDb.db.Query("SELECT COUNT(*) FROM shafiles")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var cnt int
		err = rows.Scan(&cnt)
		if err != nil {
			log.Fatal(err)
			return err
		}
		log.Printf("Count: %d", cnt)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
		return err
	}
	return nil
}

func (shaDb *ShaDatabase) getList(rootDir string) error {
	log.Printf("getList: %s\n", rootDir)
	rows, err := shaDb.db.Query("SELECT sha, path FROM shafiles WHERE path LIKE ? ORDER BY path", rootDir + "%")
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()
	for rows.Next() {
		var sha, path string
		err = rows.Scan(&sha, &path)
		if err != nil {
			log.Fatal(err)
		}
		log.Printf("%s: %s", sha, path)
		fmt.Sprintf("%s %s", sha, path)
	}
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	return nil
}

func fileShas(shaDb *ShaDatabase, rootDir string) iter.Seq2[ string, error] {
	return func (yield func( string, error) bool) {
		log.Printf("iterateShaList: %s\n", rootDir)
		rows, err := shaDb.db.Query("SELECT sha, path FROM shafiles WHERE path LIKE ? ORDER BY path", rootDir + "%")
		if err != nil {
			log.Fatal(err)
		}
		defer rows.Close()
		for rows.Next() {
			var sha, path string
			err = rows.Scan(&sha, &path)
			if err != nil {
				log.Fatal(err)
				if !yield("", err) {
					return
				}
			}
			log.Printf("%s: %s", sha, path)
			if !yield(fmt.Sprintf("%s %s", sha, path), nil) {
				return
			}
		}
		if err = rows.Err(); err != nil {
			log.Fatal(err)
			if !yield("", err) {
				return
			}
		}
		return
	}
}

func (shaDb *ShaDatabase) registerPdfWithSha (pdfPath string, pdfSha string) error {
	log.Printf("registerPdfWithSha: %s %s", pdfPath, pdfSha)

	tx, err := shaDb.db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	stmt, err := tx.Prepare("INSERT OR REPLACE INTO shafiles(sha, path) VALUES(?, ?)")
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(pdfSha, pdfPath)
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

func (shaDb *ShaDatabase) unRegisterPdf (pdfPath string) error {
	log.Printf("unRegisterPdf: %s", pdfPath)

	tx, err := shaDb.db.Begin()
	if err != nil {
		log.Fatal(err)
		return err
	}
	stmt, err := tx.Prepare("DELETE FROM shafiles WHERE path = ?")
	if err != nil {
		log.Fatal(err)
		return err
	}
	defer stmt.Close()
	_, err = stmt.Exec(pdfPath)
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

func DeletedPdf(docroot string, jpgroot string, uploadPath string) error {
	pdfPath := docroot + "/" + uploadPath
	jpgPath := strings.TrimSuffix(jpgroot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-*.jpg"
	log.Printf("DeletedPdf: %s %s %s", uploadPath, pdfPath, jpgPath)
	if err := os.Remove(pdfPath); err != nil {
		log.Println("Error removing", pdfPath, ": ", err)
		return fmt.Errorf("Error removing %s: %v", pdfPath, err)
	}
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
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	if err := shaDb.countRows(); err != nil {
		return err
	}
	if err := shaDb.getList("/"); err != nil {
		return err
	}
	if err := shaDb.unRegisterPdf(pdfPath); err != nil {
		return err
	}
	return nil
}

func GetShaList(requestedPath string) (string, error) {
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return "", fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	for line, err := range fileShas(shaDb, requestedPath) {
		if err != nil {
			return "", err
		}
		log.Printf("XX %s\n", line)
	}
	return "", nil
}

func UpdatedPdf(docroot string, jpgroot string, uploadPath string) error {
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
	pdfPath := docroot + "/" + uploadPath
	pdfSha, err := calcSha(pdfPath)
	log.Println("file hash", pdfPath, ": ", pdfSha)
	if err != nil {
		log.Printf("making calcSha: %v\n", err)
		return err
	}
	jpgPath := strings.TrimSuffix(jpgroot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-%02d.jpg"
	err = os.MkdirAll(filepath.Dir(jpgPath), os.ModePerm)
	if err != nil {
		log.Printf("making jpgpath dir: %v\n", err)
		return fmt.Errorf("making jpgpath dir: %v", err)
	}
	log.Printf("UpdatedPdf: %s %s %s", uploadPath, pdfPath, jpgPath)
	if err := createJpgFromPdf(pathExe, pdfPath, jpgPath); err != nil {
		return err
	}
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	if err := shaDb.countRows(); err != nil {
		return err
	}
	if err := shaDb.getList("/"); err != nil {
		return err
	}
	if err := shaDb.registerPdfWithSha(pdfPath, pdfSha); err != nil {
		return err
	}
	return nil
}


