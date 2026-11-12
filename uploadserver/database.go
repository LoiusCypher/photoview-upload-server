package uploadserver

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	_ "github.com/mattn/go-sqlite3"
)

const DefaultDbPath = "/sqlroot/shafiles.db"

var (
	dbPath string
	setPathOnce Once
)

// We use an enanced Once implementation because our db initialization can have errors
type Once struct {
	done uint32
	m sync.Mutex
}

func (o *Once) Do(f func() error) error {
	if atomic.LoadUint32(&o.done) == 0 {
		return o.doSlow(f)
	}
	return nil
}

func (o *Once) doSlow(f func() error) error {
	o.m.Lock()
	defer o.m.Unlock()
	var err error
	if o.done == 0 {
		err = f()
		// Only set done if no error occurred
		if err == nil {
			atomic.StoreUint32(&o.done, 1)
		}
	}
	return err
}

func getDbPath() *string {
	setPathOnce.Do(func() error {
		err := initDb( DefaultDbPath)
		if err == nil {
			dbPath = DefaultDbPath
		}
		return err
	})
	return &dbPath
}

func SetDbPath( sqlPath string) *string {
	setPathOnce.Do(func() error {
		err := initDb( sqlPath)
		if err == nil {
			dbPath = sqlPath
			log.Printf("success SetDbPath: %s %s\n", sqlPath, dbPath)
		} else {
			log.Printf("failed SetDbPath: %s %v\n", sqlPath, err)
		}
		return err
	})
	return &dbPath
}

func initDb( sqlPath string) error {
	// log.Printf("initDb: %s", sqlPath)
	// os.Remove(sqlPath)
	if err := os.MkdirAll(filepath.Dir(sqlPath), os.ModePerm); err != nil {
		log.Printf("making sqlPath dir: %v\n", err)
		return fmt.Errorf("making sqlPath dir: %v", err)
	}
	db, err := sql.Open("sqlite3", sqlPath)
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

func openDb() *sql.DB {
	// log.Printf("openDb: %s", sqlPath)
	sqlPath := getDbPath()
	if dbPath == ""{
		log.Println( fmt.Errorf("could not init db"))
		return nil
	}
	db, err := sql.Open("sqlite3", *sqlPath)
	if err != nil {
		log.Println( fmt.Errorf("open db: %v", err))
		return nil
	}
	var cnt int
	if err := db.QueryRow("SELECT COUNT(*) FROM shafiles;").Scan(&cnt); err != nil {
		if err != sql.ErrNoRows {
			log.Println( fmt.Errorf("count shafiles: %v", err))
			return nil
		}
	}
	log.Printf("Count: %d", cnt)
	if err != nil {
		log.Println( fmt.Errorf("count rows: %v", err))
		return nil
	}
	return db
}

type FileInfo struct {
	Path string `json:"path"`
	Sha  string `json:"sha"`
}

func listShas( listPath string) ( []FileInfo, error) {
	log.Printf("listShas: %s\n", listPath)
	var fileInfos []FileInfo
	shaDb := openDb()
	if shaDb == nil {
		return fileInfos, fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	rows, err := shaDb.Query("SELECT sha, path FROM shafiles WHERE path LIKE ? ORDER BY path", listPath + "%")
	if err != nil {
		log.Println( fmt.Errorf("query: %v", err))
		return fileInfos, err
	}
	defer rows.Close()
	for rows.Next() {
		var fileInfo FileInfo
		err = rows.Scan(&fileInfo.Sha, &fileInfo.Path)
		if err != nil {
			log.Println( fmt.Errorf("query: %v", err))
			return fileInfos, err
		}
		// log.Printf("DB LIST %s: %s", fileInfo.Sha, fileInfo.Path)
		fileInfos = append( fileInfos, fileInfo)
		// fmt.Fprintln(w, fileInfo.Sha, fileInfo.Path)
	}
	if err = rows.Err(); err != nil {
		log.Println( fmt.Errorf("query: %v", err))
		return fileInfos, err
	}
	return fileInfos, nil
}

func getPathSha( reqPath string) ( string, error) {
	// log.Printf("getPathSha: %s\n", reqPath)
	shaDb := openDb()
	if shaDb == nil {
		return "", fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	var sha string
	if err := shaDb.QueryRow("SELECT sha FROM shafiles WHERE path = ?;", reqPath).Scan(&sha); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		return "", err
	}
	// log.Printf("getPathSha: %s\n", sha)
	return sha, nil
}

func recordSha(filePath string, registerPath string) ( string, error) {
	fileSha, err := calcSha(filePath)
	if err != nil {
		log.Printf("making calcSha: %v\n", err)
		return "", err
	}
	// log.Println("file hash", filePath, ": ", fileSha)
	shaDb := openDb()
	if shaDb == nil {
		return "", fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	tx, err := shaDb.Begin()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	_, err = tx.Exec("INSERT OR REPLACE INTO shafiles(sha, path) VALUES(?, ?)", fileSha, registerPath)
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	return fileSha, nil
}

func deleteSha(registerPath string) ( string, error) {
	log.Printf("deleteSha: %s", registerPath)
	shaDb := openDb()
	if shaDb == nil {
		return "", fmt.Errorf("openDb failed")
	}
	defer shaDb.Close()
	tx, err := shaDb.Begin()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	var sha string
	if err := tx.QueryRow("DELETE FROM shafiles WHERE path = ? RETURNING sha", registerPath).Scan(&sha); err != nil {
		if err == sql.ErrNoRows {
			return "", nil
		}
		log.Fatal(err)
		return "", err
	}
	err = tx.Commit()
	if err != nil {
		log.Fatal(err)
		return "", err
	}
	return sha, nil
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

