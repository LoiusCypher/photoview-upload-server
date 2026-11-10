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

func deleteJpgs(jpgroot string, uploadPath string) error {
	jpgPath := strings.TrimSuffix(jpgroot + "/" + uploadPath, filepath.Ext(uploadPath)) + "-*.jpg"
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

