package uploadserver

import (
	"testing"
	"github.com/spf13/afero"
	"github.com/stretchr/testify/assert"
	"github.com/jung-kurt/gofpdf"
)

func CreatePdf_TestHelper(t *testing.T, content []string, pdfFilePath string) {
        t.Helper()

	pdf := gofpdf.New("P", "mm", "A4", "")
	for _, text := range content {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, text)
	}
	pdf.OutputFileAndClose( pdfFilePath)
}

func TestPdfFiles(t *testing.T) {
	// =========================================Setup Database
	//fs := afero.NewMemMapFs()
	fs := afero.NewOsFs()
	// =========================================Add a file
	pdfRoot := "/tmp/app/pdfs"
	err := fs.MkdirAll( pdfRoot, 0755)
	assert.Nil( t, err)
	jpgRoot := "/tmp/app/jpgs"
	err = fs.MkdirAll( jpgRoot, 0755)
	assert.Nil( t, err)
	// =========================================convert a file
	pdfPath_Empty := pdfRoot + "/" + "uploadedEmpty.pdf"
	CreatePdf_TestHelper( t, []string{ "",}, pdfPath_Empty)
	jpgPath_Empty := "path/subdir/" + "Empty.pdf"
	err1 := convertPdfToJpg( pdfPath_Empty, jpgRoot, jpgPath_Empty)
	assert.Nil( t, err1)
	// =========================================
}

