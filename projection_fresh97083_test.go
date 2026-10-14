package cibudgetdomain

import (
	"encoding/json"
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestSourceOwnedPolicyProjection97083(t *testing.T) {
	raw, err := os.ReadFile("origins/policy-projections.json")
	if err != nil {
		t.Fatal(err)
	}
	var rows []struct {
		Activity, Source, Generated string
		SourceSHA                   string `json:"source_sha256"`
		GeneratedSHA                string `json:"generated_sha256"`
	}
	if err = json.Unmarshal(raw, &rows); err != nil || len(rows) == 0 {
		t.Fatal(rows, err)
	}
	current, err := os.ReadFile("policy.gooo")
	if err != nil {
		t.Fatal(err)
	}
	declarations := regexp.MustCompile(`(?m)^activity ([A-Za-z_][A-Za-z0-9_]*)\(`).FindAllStringSubmatch(string(current), -1)
	declared := map[string]bool{}
	for _, match := range declarations {
		if declared[match[1]] {
			t.Fatal("repeated source activity")
		}
		declared[match[1]] = true
	}
	if len(declared) != len(rows) {
		t.Fatal("source policy and projection populations differ")
	}
	for _, row := range rows {
		if !declared[row.Activity] {
			t.Fatal("undeclared or repeated projection", row.Activity)
		}
		delete(declared, row.Activity)
		source, err := os.ReadFile(row.Source)
		if err != nil || hashBytes(source) != row.SourceSHA {
			t.Fatal(row.Activity, "source origin differs", err)
		}
		generated, err := os.ReadFile(row.Generated)
		if err != nil || hashBytes(generated) != row.GeneratedSHA {
			t.Fatal(row.Activity, "generated region differs", err)
		}
		needle := "activity " + row.Activity + "("
		start := strings.Index(string(source), needle)
		if start < 0 {
			t.Fatal(row.Activity)
		}
		body := string(source)[start:]
		body = body[:strings.Index(body, "computes `")+len("computes `")]
		rest := string(source)[start+len(body):]
		declaration := body + rest[:strings.Index(rest, "`")+1]
		if !strings.Contains(string(current), declaration) {
			t.Fatal(row.Activity, "active source declaration differs from its generation origin")
		}
	}
	if SourcePolicyDigest() != hashBytes(current) || ReserveCost(951, 339) != 1290 {
		t.Fatal("source-owned reserve projection differs")
	}
}
