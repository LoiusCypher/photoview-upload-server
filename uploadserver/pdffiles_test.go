package uploadserver

import (
	"fmt"
	"os"
	"strings"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/suyuan32/gofpdf-next/fpdf"
)

func CreatePdfFile_TestHelper(t *testing.T, contentPages []string, pdfRoot string) string {
        t.Helper()

	f, err1 := os.CreateTemp( pdfRoot, "*.pdf")
	assert.Nil( t, err1, fmt.Sprintf( "failed to create pdf dir: %v", err1))
	defer f.Close()
	pdfFilePath := f.Name()
	// pdf := gofpdf.New("P", "mm", "A4", "")
	pdf := fpdf.New("P", "mm", "A4", "")
	for _, text := range contentPages {
		pdf.AddPage()
		pdf.SetFont("Arial", "B", 16)
		pdf.Cell(40, 10, text)
	}
	pdf.OutputFileAndClose( pdfFilePath)
	return pdfFilePath
}

func TestPdfFiles(t *testing.T) {
	// =========================================Add a file
	pdfRoot := "/tmp/app/pdfs"
	err := os.MkdirAll( pdfRoot, 0755)
	assert.Nil( t, err)
	jpgRoot := "/tmp/app/jpgs"
	err = os.MkdirAll( jpgRoot, 0755)
	assert.Nil( t, err)
	// =========================================convert a file
	pdfPath_Empty := CreatePdfFile_TestHelper( t, []string{ "",}, pdfRoot)
	jpgPath_Empty := "path/subdir/" + "Empty.pdf"
	err1 := convertPdfToJpg( pdfPath_Empty, jpgRoot, jpgPath_Empty)
	assert.Nil( t, err1)
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Empty, ".pdf") + "-01.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Empty, ".pdf") + "-02.jpg")
	// =========================================
	pdfPath_Multi := CreatePdfFile_TestHelper( t, []string{ "Multi 1", "Multi 2", "Multi 3", "Multi 4", "Multi 5",}, pdfRoot)
	jpgPath_Multi := "path/subdir/" + "Multi.pdf"
	err2 := convertPdfToJpg( pdfPath_Multi, jpgRoot, jpgPath_Multi)
	assert.Nil( t, err2)
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-01.jpg")
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-02.jpg")
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-03.jpg")
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-04.jpg")
	assert.FileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-05.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-06.jpg")
	// =========================================
	err3 := deleteJpgs( jpgRoot, jpgPath_Multi)
	assert.Nil( t, err3)
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-01.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-02.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-03.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-04.jpg")
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Multi, ".pdf") + "-05.jpg")
	// =========================================
	err4 := deleteJpgs( jpgRoot, jpgPath_Empty)
	assert.Nil( t, err4)
	assert.NoFileExists( t, jpgRoot + "/" + strings.TrimSuffix( jpgPath_Empty, ".pdf") + "-01.jpg")
	// =========================================
}

