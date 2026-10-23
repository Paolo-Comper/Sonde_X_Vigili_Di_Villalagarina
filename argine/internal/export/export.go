// Package export serializza i risultati SQL in CSV/JSON e valida le query
// read-only eseguibili dalla dashboard.
package export

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"argine/internal/store"
)

// ValidateSelect accetta una sola istruzione read-only.
func ValidateSelect(query string) error {
	s := strings.TrimSpace(query)
	if s == "" {
		return errors.New("query vuota")
	}
	for strings.HasSuffix(s, ";") {
		s = strings.TrimSpace(strings.TrimSuffix(s, ";"))
	}
	if strings.Contains(s, ";") {
		return errors.New("sono consentite solo query singole (rimuovi i ';')")
	}
	upper := strings.ToUpper(s)
	if !strings.HasPrefix(upper, "SELECT") && !strings.HasPrefix(upper, "WITH") {
		return errors.New("sono consentite solo query SELECT (o WITH ... SELECT)")
	}
	for _, kw := range []string{
		"INSERT ", "UPDATE ", "DELETE ", "DROP ", "ALTER ", "CREATE ",
		"ATTACH ", "DETACH ", "PRAGMA ", "REPLACE ", "VACUUM", "TRIGGER ", "REINDEX",
	} {
		if strings.Contains(upper, kw) {
			return fmt.Errorf("parola chiave non consentita: %s", strings.TrimSpace(kw))
		}
	}
	return nil
}

// Table e' un result set materializzato in memoria.
type Table struct {
	Columns []string
	Rows    [][]any
	IDs     []int64
}

// Materialize legge tutte le righe e raccoglie gli id (se la colonna "id" c'e')
// cosi' da poter marcare i record esportati.
func Materialize(rows store.Rows) (*Table, error) {
	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	t := &Table{Columns: cols}
	idIdx := columnIndex(cols, "id")

	err = each(rows, func(_ int, vals []any) error {
		row := make([]any, len(vals))
		copy(row, vals)
		t.Rows = append(t.Rows, row)
		if idIdx >= 0 {
			if id, ok := asInt64(vals[idIdx]); ok {
				t.IDs = append(t.IDs, id)
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return t, nil
}

// WriteCSV scrive l'header e le righe in formato CSV.
func (t *Table) WriteCSV(w io.Writer) error {
	if _, err := io.WriteString(w, csvLine(t.Columns)); err != nil {
		return err
	}
	for _, row := range t.Rows {
		fields := make([]string, len(row))
		for i, v := range row {
			fields[i] = csvField(v)
		}
		if _, err := io.WriteString(w, csvLine(fields)); err != nil {
			return err
		}
	}
	return nil
}

// WriteJSON scrive un array JSON di oggetti.
func (t *Table) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	recs := make([]map[string]any, 0, len(t.Rows))
	for _, row := range t.Rows {
		rec := make(map[string]any, len(t.Columns))
		for i, c := range t.Columns {
			rec[c] = normalize(row[i])
		}
		recs = append(recs, rec)
	}
	return enc.Encode(recs)
}

// Records restituisce al massimo limit righe come mappe (anteprima).
func (t *Table) Records(limit int) []map[string]any {
	n := len(t.Rows)
	if limit > 0 && n > limit {
		n = limit
	}
	out := make([]map[string]any, 0, n)
	for _, row := range t.Rows[:n] {
		rec := make(map[string]any, len(t.Columns))
		for i, c := range t.Columns {
			rec[c] = normalize(row[i])
		}
		out = append(out, rec)
	}
	return out
}

func each(rows store.Rows, fn func(int, []any) error) error {
	cols, err := rows.Columns()
	if err != nil {
		return err
	}
	vals := make([]any, len(cols))
	ptrs := make([]any, len(cols))
	for i := range vals {
		ptrs[i] = &vals[i]
	}
	n := 0
	for rows.Next() {
		if err := rows.Scan(ptrs...); err != nil {
			return err
		}
		if err := fn(n, vals); err != nil {
			return err
		}
		n++
	}
	return rows.Err()
}

func columnIndex(cols []string, name string) int {
	for i, c := range cols {
		if strings.EqualFold(c, name) {
			return i
		}
	}
	return -1
}

func csvLine(fields []string) string {
	return strings.Join(fields, ",") + "\n"
}

func csvField(v any) string {
	s := normalizeString(v)
	if strings.ContainsAny(s, ",\"\n\r") {
		return "\"" + strings.ReplaceAll(s, "\"", "\"\"") + "\""
	}
	return s
}

func normalizeString(v any) string {
	switch x := v.(type) {
	case nil:
		return ""
	case []byte:
		return string(x)
	case string:
		return x
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64)
	case float32:
		return strconv.FormatFloat(float64(x), 'f', -1, 64)
	case time.Time:
		return x.Format(time.RFC3339)
	default:
		return fmt.Sprint(x)
	}
}

func normalize(v any) any {
	if b, ok := v.([]byte); ok {
		return string(b)
	}
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339)
	}
	return v
}

func asInt64(v any) (int64, bool) {
	switch x := v.(type) {
	case int64:
		return x, true
	case int:
		return int64(x), true
	case float64:
		return int64(x), true
	case []byte:
		n, err := strconv.ParseInt(string(x), 10, 64)
		return n, err == nil
	}
	return 0, false
}
