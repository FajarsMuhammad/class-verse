package gradebook_test

import (
	"bytes"
	"encoding/csv"
	"testing"

	"github.com/FajarsMuhammad/class-verse/classverse-backend/internal/gradebook"
)

func TestCSVFormulaSafety(t *testing.T) {
	for _, input := range []string{"=SUM(1,2)", "+1", "-2", "@name", " \t=cmd", "\r\n@evil", "\ufeff=bad"} {
		if got := gradebook.CSVCell(input); got != "'"+input {
			t.Errorf("unsafe %q became %q", input, got)
		}
	}
	for _, input := range []string{"Ani", "", " Siswa", "12"} {
		if gradebook.CSVCell(input) != input {
			t.Errorf("unnecessary change %q", input)
		}
	}
	var out bytes.Buffer
	if err := gradebook.WriteCSV(&out, []gradebook.Row{{Name: "=HYPERLINK(\"x\")", ClassName: "XI, A"}}); err != nil {
		t.Fatal(err)
	}
	rows, err := csv.NewReader(&out).ReadAll()
	if err != nil || rows[1][0] != "'=HYPERLINK(\"x\")" || rows[1][1] != "XI, A" || rows[1][6] != "" {
		t.Fatalf("CSV=%v %v", rows, err)
	}
}
