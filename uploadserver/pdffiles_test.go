package uploadserver

import (
/*
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"reflect"
	"strings"
*/
	//"errors"
	//"net/http/httptest"
	//"os"
	"path"
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/jung-kurt/gofpdf"
)

func CreatePdf_TestHelper(t *testing.T, content string, pdfFilePath string) {
        t.Helper()

	pdf := gofpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Arial", "B", 16)
	pdf.Cell(40, 10, content)
	pdf.OutputFileAndClose( pdfFilePath)
}

func TestPdfFiles(t *testing.T) {
	// =========================================Setup Database
	// sqlPath := "/sqlroot/shafiles.db"
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	//var fileInfos []FileInfo
	// =========================================Add a file
	pdfRoot := "/pdfs"
	err = fs.MkdirAll( pdfRoot), 0755)
	assert.Nil( t, err)
	jpgRoot := "/jpgs"
	err = fs.MkdirAll( jpgRoot), 0755)
	assert.Nil( t, err)
	// =========================================convert a file
	pdfPath_Empty := pdfRoot + "/" + "uploadedEmpty.pdf 
	CreatePdf_TestHelper( t, "", pdfPath_Empty)
	jpgPath_Empty := jpgRoot + "/path/subdir/" + "Empty.pdf 
	err1 := convertPdfToJpg( pdfPath_Empty, jpgRoot, jpgPath_Empty)
	assert.Nil( t, err1)
	// =========================================
}

