package uploadserver

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func convertPdfToJpg( uploadedPath string, jpgRoot string, destPath string) error {
	var targExe string
	var pathExe string
	// targExe = "gs"
	targExe = "ls"
	pathExe, err := exec.LookPath(targExe)
	if err != nil {
		log.Printf("didn't find '%s' executable\n", targExe)
		return fmt.Errorf("didn't find %s: %v", targExe, err)
	}
	// log.Printf("'%s' executable in '%s'\n", targExe, pathExe)
	jpgPattern := strings.TrimSuffix(jpgRoot + "/" + destPath, filepath.Ext(destPath)) + "-%02d.jpg"
	err = os.MkdirAll(filepath.Dir(jpgPattern), 0777) // os.ModePerm)
	log.Printf("convertPdfToJpg: jpg dir created %s", filepath.Dir(jpgPattern))
	if err != nil {
		log.Printf("making jpgpath dir: %v\n", err)
		return fmt.Errorf("making jpgpath dir: %v", err)
	}
	log.Printf("convertPdfToJpg: %s %s", uploadedPath, jpgPattern)
	// cmdArgs := []string{ "-dNOPAUSE", "-sDEVICE=jpeg", "-r100", "-sOutputFile=" + jpgPattern, "-f", uploadedPath, "-c", "quit"}
	cmdArgs := []string{ "-al", "/", "/opt",  "/opt/app", "/opt/app/jpgs", "/opt/app/pdfs", filepath.Dir(jpgPattern), filepath.Dir(uploadedPath)}
	cmd := exec.Command(pathExe, cmdArgs...)
	cmd.Dir = filepath.Dir(uploadedPath)
	var out strings.Builder
	var sterr strings.Builder
	log.Println("cmd:", pathExe, cmdArgs)
	cmd.Stdout = &out
	cmd.Stderr = &sterr
	if err := cmd.Run(); err != nil {
		log.Println("cmd workdir:", filepath.Dir(uploadedPath))
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
	log.Printf("stdout: %q\n", out.String())
	log.Printf("stderr: %q\n", sterr.String())
	return nil
}

func deleteJpgs(jpgRoot string, uploadPath string) error {
	jpgPath := strings.TrimSuffix(jpgRoot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-*.jpg"
	log.Printf("deleteJpgs: %s %s", uploadPath, jpgPath)
	jpgFiles, err := filepath.Glob(jpgPath)
	if err != nil {
		log.Println("Error resolving file pattern", jpgPath, ": ", err)
		return fmt.Errorf("resolving file pattern %s: %v", jpgPath, err)
	}
	for _, jpgFile := range jpgFiles {
		if err := os.Remove(jpgFile); err != nil {
			log.Println("Error removing", jpgFile, ": ", err)
			return fmt.Errorf("removing %s: %v", jpgFile, err)
		}
	}
	return nil
}

