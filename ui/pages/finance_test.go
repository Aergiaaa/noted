package pages

import (
	"encoding/json"
	"strings"
	"testing"

	database "github.com/Aergiaaa/noted/internal/database"
	"github.com/jackc/pgx/v5/pgtype"
)

type rowInput struct {
	id     string
	title  string
	kind   string
	amount string
	date   string
	from   string
	to     string
}

func inputToRow(in rowInput) database.GetTransactionsWithPocketNamesRow {
	row := database.GetTransactionsWithPocketNamesRow{
		Title: in.title,
		Type:  in.kind,
	}

	_ = row.ID.Scan(in.id)
	_ = row.Amount.Scan(in.amount)
	_ = row.Date.Scan(in.date)

	if in.from != "" {
		_ = row.FromPocketID.Scan(in.from)
	}

	if in.to != "" {
		_ = row.ToPocketID.Scan(in.to)
	}

	return row
}

func TestTransactionRowData(t *testing.T) {
	tests := []struct {
		name     string
		row      rowInput
		expected map[string]any
	}{
		{
			name: "expense with from pocket",
			row: rowInput{
				id:     "b54c00db-827a-4a17-b0d0-4d81925b20c7",
				title:  "Makan",
				kind:   "expense",
				amount: "25000.50",
				date:   "2026-08-14",
				from:   "b54c00db-827a-4a17-b0d0-4d81925b20c7",
			},
			expected: map[string]any{
				"id":     "b54c00db-827a-4a17-b0d0-4d81925b20c7",
				"title":  "Makan",
				"type":   "expense",
				"amount": 25000.5,
				"date":   "2026-08-14",
				"from":   "b54c00db-827a-4a17-b0d0-4d81925b20c7",
				"to":     "",
			},
		},
		{
			name: "income with to pocket",
			row: rowInput{
				id:     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
				title:  "Gaji",
				kind:   "income",
				amount: "5000000",
				date:   "2026-08-15",
				to:     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
			},
			expected: map[string]any{
				"id":     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
				"title":  "Gaji",
				"type":   "income",
				"amount": 5000000.0,
				"date":   "2026-08-15",
				"from":   "",
				"to":     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
			},
		},
		{
			name: "transfer with both pockets",
			row: rowInput{
				id:     "c7d4a8cc-a5c7-4d47-5eae-56da2db182b4",
				title:  "Pindah",
				kind:   "transfer",
				amount: "100000",
				date:   "2026-08-13",
				from:   "b54c00db-827a-4a17-b0d0-4d81925b20c7",
				to:     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
			},
			expected: map[string]any{
				"id":     "c7d4a8cc-a5c7-4d47-5eae-56da2db182b4",
				"title":  "Pindah",
				"type":   "transfer",
				"amount": 100000.0,
				"date":   "2026-08-13",
				"from":   "b54c00db-827a-4a17-b0d0-4d81925b20c7",
				"to":     "a8cc0a05-c7d4-475e-ae56-da2db182b4ff",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := transactionRowData(inputToRow(tt.row))
			if got == "" {
				t.Fatalf("expected non-empty row data")
			}

			var decoded map[string]any
			if err := json.Unmarshal([]byte(got), &decoded); err != nil {
				t.Fatalf("row data is not valid json: %v", err)
			}

			for k, want := range tt.expected {
				val, ok := decoded[k]
				if !ok {
					t.Fatalf("missing key %q in row data %s", k, got)
				}
				if val != want {
					t.Fatalf("key %q = %v, want %v", k, val, want)
				}
			}
		})
	}
}

func TestTransactionRowDataEscapes(t *testing.T) {
	got := transactionRowData(inputToRow(rowInput{
		id:     "b54c00db-827a-4a17-b0d0-4d81925b20c7",
		title:  `Go "quoted" <<note>>`,
		kind:   "expense",
		amount: "10",
		date:   "2026-08-14",
	}))

	if strings.Contains(got, "<") || strings.Contains(got, "\n") {
		t.Fatalf("row data must be JSON-safe, got %q", got)
	}

	if !strings.Contains(got, `Go \"quoted\"`) {
		t.Fatalf("title must be escaped in json, got %q", got)
	}
}

func TestTransactionTagsProps(t *testing.T) {
	got := transactionTagsProps(`b54c00db-827a-4a17-b0d0-4d81925b20c7"`)

	want := `transactionTags({id: "b54c00db-827a-4a17-b0d0-4d81925b20c7\""})`

	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestPocketRowData(t *testing.T) {
	row := database.GetPocketBalancesRow{}
	_ = row.ID.Scan("b54c00db-827a-4a17-b0d0-4d81925b20c7")
	_ = row.Balance.Scan("25000")
	row.Name = `BCA "Utama"`

	got := pocketRowData(row)

	var decoded map[string]any
	if err := json.Unmarshal([]byte(got), &decoded); err != nil {
		t.Fatalf("expected valid json, got %q: %v", got, err)
	}

	if decoded["id"] != "b54c00db-827a-4a17-b0d0-4d81925b20c7" {
		t.Fatalf("expected id, got %v", decoded["id"])
	}

	if decoded["name"] != `BCA "Utama"` {
		t.Fatalf("expected name, got %v", decoded["name"])
	}

	if decoded["type"] != "" {
		t.Fatalf("expected type, got %v", decoded["type"])
	}

	if strings.Contains(got, "<") || strings.Contains(got, "\n") {
		t.Fatalf("row data must be JSON-safe, got %q", got)
	}
}

func TestSignedNet(t *testing.T) {
	var pos, neg pgtype.Numeric
	_ = pos.Scan("12.50")
	_ = neg.Scan("-4.25")

	if got := signedNet(pos); got != "+12.50" {
		t.Fatalf("expected +12.50, got %q", got)
	}

	if got := signedNet(neg); got != "-4.25" {
		t.Fatalf("expected -4.25, got %q", got)
	}
}
