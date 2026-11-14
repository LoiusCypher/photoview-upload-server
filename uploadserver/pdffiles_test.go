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
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Empty, ".pdf") + "-01.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Empty, ".pdf") + "-02.pdf")
	// =========================================
	pdfPath_Multi := pdfRoot + "/" + "uploadedMulti.pdf"
	CreatePdf_TestHelper( t, []string{ "Multi 1", "Multi 2", "Multi 3", "Multi 4", "Multi 5",}, pdfPath_Multi)
	jpgPath_Multi := "path/subdir/" + "Multi.pdf"
	err2 := convertPdfToJpg( pdfPath_Multi, jpgRoot, jpgPath_Multi)
	assert.Nil( t, err2)
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-01.pdf")
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-02.pdf")
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-03.pdf")
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-04.pdf")
	assert.FileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-05.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-06.pdf")
	// =========================================
	err3 := deleteJpgs( jpgRoot, jpgPath_Multi)
	assert.Nil( t, err3)
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-01.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-02.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-03.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-04.pdf")
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Multi, ".pdf") + "-05.pdf")
	// =========================================
	err4 := deleteJpgs( jpgRoot, jpgPath_Empty)
	assert.Nil( t, err1)
	assert.NotFileExists( t, jpgRoot + "/" + strinds.TrimSuffix( jpgPath_Empty, ".pdf") + "-01.pdf")
	// =========================================
}

