package gradebook

import (
	"encoding/csv"
	"io"
	"strconv"
	"strings"
	"unicode"
)

// CSVCell neutralizes spreadsheet formulas, even after invisible leading characters.
func CSVCell(value string) string {
	first := strings.TrimLeftFunc(value, func(r rune) bool { return unicode.IsSpace(r) || unicode.IsControl(r) || r == '\ufeff' })
	if first != "" && strings.ContainsRune("=+-@", []rune(first)[0]) {
		return "'" + value
	}
	return value
}

// WriteCSV uses the same rows as the filtered JSON report, no personal contact details or quiz keys.
func WriteCSV(w io.Writer, rows []Row) error {
	c := csv.NewWriter(w)
	c.UseCRLF = true
	if err := c.Write([]string{"Nama", "Kelas pada tahun terpilih", "Modul selesai", "Modul tersedia", "Kuis dikerjakan", "Kuis tersedia", "Rata-rata nilai terbaik"}); err != nil {
		return err
	}
	for _, r := range rows {
		avg := ""
		if r.AverageBest != nil {
			avg = strconv.FormatFloat(*r.AverageBest, 'f', 2, 64)
		}
		if err := c.Write([]string{CSVCell(r.Name), CSVCell(r.ClassName), strconv.Itoa(int(r.ModulesDone)), strconv.Itoa(int(r.ModulesTotal)), strconv.Itoa(int(r.QuizzesAttempted)), strconv.Itoa(int(r.QuizzesTotal)), avg}); err != nil {
			return err
		}
	}
	c.Flush()
	return c.Error()
}
