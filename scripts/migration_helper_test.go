package scripts_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

func TestApplySQLFileHelperStopsOnStatementErrors(t *testing.T) {
	if _, err := exec.LookPath("bash"); err != nil {
		t.Skip("bash is not available")
	}

	repoRoot, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	helper := filepath.Join(repoRoot, "scripts", "apply-sql-file.sh")
	if _, err := os.Stat(helper); err != nil {
		t.Fatalf("missing SQL helper: %v", err)
	}

	tmp := t.TempDir()
	fakeDocker := filepath.Join(tmp, "docker")
	if runtime.GOOS == "windows" {
		fakeDocker = filepath.Join(tmp, "docker")
	}
	if err := os.WriteFile(fakeDocker, []byte(`#!/usr/bin/env bash
sql="$(cat)"
strict=0
if [[ " $* " == *" -v ON_ERROR_STOP=1 "* && " $* " == *" --single-transaction "* ]]; then
  strict=1
fi
if [[ "$sql" == *"SELECT 1/0"* && "$strict" == "1" ]]; then
  exit 1
fi
exit 0
`), 0755); err != nil {
		t.Fatal(err)
	}

	sqlFile := filepath.Join(tmp, "failing.sql")
	if err := os.WriteFile(sqlFile, []byte("SELECT 1;\nSELECT 1/0;\nSELECT 2;\n"), 0644); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command("bash", helper, sqlFile)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "PATH="+tmp+string(os.PathListSeparator)+os.Getenv("PATH"))
	output, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("helper succeeded for failing SQL; output:\n%s", output)
	}
}
