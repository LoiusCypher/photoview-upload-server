package uploadserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	_ "github.com/mattn/go-sqlite3"
)

type ShaDatabase struct {
	sqlPath string
	db *sql.DB
}

func (shaDb *ShaDatabase) Close() {
	shaDb.db.Close()
}

func init() {
	// log.Printf("init()")
	// os.Remove("/sqlroot/shafiles.db")
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		log.Println( fmt.Errorf("openDb failed"))
		return
	}
	defer shaDb.Close()
	err := shaDb.countRows()
	if err != nil {
		log.Println( fmt.Errorf("count rows: %v", err))
		return
	}
}

func openDb (sqlPath string) *ShaDatabase {
	// log.Printf("openDb: %s", sqlPath)
	if err := initDb(sqlPath); err != nil {
		log.Println( fmt.Errorf("init db: %v", err))
		return nil
	}

	db, err := sql.Open("sqlite3", sqlPath)
	if err != nil {
		log.Println( fmt.Errorf("open db: %v", err))
		return nil
	}
	shaDb := &ShaDatabase{ sqlPath, db, }
	return shaDb
}

func initDb (sqlPath string) error {
	// log.Printf("initDb: %s", sqlPath)
	// os.Remove("/sqlroot/shafiles.db")
	if err := os.MkdirAll(filepath.Dir(sqlPath), os.ModePerm); err != nil {
		log.Printf("making sqlPath dir: %v\n", err)
		return fmt.Errorf("making sqlPath dir: %v", err)
	}
	db, err := sql.Open("sqlite3", "/sqlroot/shafiles.db")
	if err != nil {
		log.Println( fmt.Errorf("open db: %v", err))
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

func ListShas( w http.ResponseWriter, listPath string) ( bool, error) {
	log.Printf("ListShas: %s\n", listPath)
	shaDb := openDb("/sqlroot/shafiles.db")
	if shaDb == nil {
		return false, fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	rows, err := shaDb.db.Query("SELECT sha, path FROM shafiles WHERE path LIKE ? ORDER BY path", listPath + "%")
	if err != nil {
		log.Println( fmt.Errorf("query: %v\n", err))
		return false, err
	}
	defer rows.Close()
	found := false
	for rows.Next() {
		var sha, path string
		err = rows.Scan(&sha, &path)
		if err != nil {
			log.Println( fmt.Errorf("query: %v\n", err))
			return false, err
		}
		// log.Printf("DB LIST %s: %s", sha, path)
		fmt.Fprintln(w, sha, path)
		found = true
	}
	if err = rows.Err(); err != nil {
		log.Println( fmt.Errorf("query: %v", err))
		return false, err
	}
	return found, nil
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

func recordSha(pdfPath string, registerPath string) error {
	pdfSha, err := calcSha(pdfPath)
	if err != nil {
		log.Printf("making calcSha: %v\n", err)
		return err
	}
	// log.Println("file hash", pdfPath, ": ", pdfSha)
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

func calcSha(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		log.Println("Error opening file", filePath, ": ", err)
		return "", fmt.Errorf("opening file %s: %v", filePath, err)
	}
	defer f.Close()
	hasher := sha256.New()
	if _, err := io.Copy(hasher, f); err != nil {
		log.Println("Error calculating file hash", filePath, ": ", err)
		return "", fmt.Errorf("calculating file hash %s: %v", filePath, err)
	}
	// log.Println("file hash", filePath, ": ", hex.EncodeToString(hasher.Sum(nil)))
	return hex.EncodeToString(hasher.Sum(nil)), nil
}

