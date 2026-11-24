package uploadserver

import (
	"fmt"
	"os"
	"path"
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func Test_PdfFiles(t *testing.T) {
	jpgRoot := "/jpgs"
	pattern := "_%02.jpg"
	tests := []struct {
		title    string
		pdfPath  string
		want     string
	}{
		{ "Simple file","fooBar.pdf", path.Join( jpgRoot, "fooBar",  "fooBar" + pattern), },
		{ "File in subdir", "foo/fooBar.pdf", path.Join( jpgRoot, "foo/fooBar",  "fooBar" + pattern), },
		{ "Absolute path", "/bar/fooBar.pdf", path.Join( jpgRoot, "bar/fooBar",  "fooBar" + pattern), },
		{ "Relative path leaving scope", "../fooBar.pdf", path.Join( jpgRoot, "fooBar",  "fooBar" + pattern) , },
		{ "Absolute path leaving scope", "/../fooBar.pdf", path.Join( jpgRoot, "fooBar",  "fooBar" + pattern) , },
		{ "File without .pdf extension", "fooBar", path.Join( jpgRoot, "fooBar",  "fooBar" + pattern), },
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			if got := convertPdfToJpgFilename( jpgRoot, tt.pdfPath, pattern); got != tt.want {
				t.Errorf("convertPdfToJpgFilename(%s) = %v, want %v", tt.pdfPath, got, tt.want)
			}
		})
	}
	// =========================================Add a file
	pdfRoot, err1 := os.MkdirTemp("", "")
	require.Nil(t, err1)
	defer os.RemoveAll( pdfRoot)

	jpgRoot, err2 := os.MkdirTemp("", "")
	require.Nil(t, err2)
	defer os.RemoveAll( jpgRoot)

	jpgPath_Empty := "path/subdir/" + "Empty.pdf"
	jpgPath_Multi := "path/subdir/" + "Multi.pdf"

	// =========================================convert a file
	t.Run( "Convert single page PDF", func(t *testing.T) {
		pdfPath_Empty := CreatePdfFile_TestHelper( t, []string{ "",}, pdfRoot)
		err1 := convertPdfToJpg( pdfPath_Empty, jpgRoot, jpgPath_Empty)
		assert.Nil( t, err1)
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Empty, "-01.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Empty, "-02.jpg"))
	})
	// =========================================
	t.Run( "Convert multi page PDF", func(t *testing.T) {
		pdfPath_Multi := CreatePdfFile_TestHelper( t, []string{ "Multi 1", "Multi 2", "Multi 3", "Multi 4", "Multi 5",}, pdfRoot)
		err2 := convertPdfToJpg( pdfPath_Multi, jpgRoot, jpgPath_Multi)
		assert.Nil( t, err2)
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-01.jpg"))
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-02.jpg"))
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-03.jpg"))
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-04.jpg"))
		assert.FileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-05.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-06.jpg"))
	})
	// =========================================
	t.Run( "Delete multi page PDF", func(t *testing.T) {
		err3 := deleteJpgs( jpgRoot, jpgPath_Multi)
		assert.Nil( t, err3)
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-01.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-02.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-03.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-04.jpg"))
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Multi, "-05.jpg"))
	})
	// =========================================
	t.Run( "Delete single page PDF", func(t *testing.T) {
		err4 := deleteJpgs( jpgRoot, jpgPath_Empty)
		assert.Nil( t, err4)
		assert.NoFileExists( t, convertPdfToJpgFilename( jpgRoot, jpgPath_Empty, "-01.jpg"))
	})
	// =========================================
}

