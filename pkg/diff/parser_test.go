package diff

import (
	"strings"
	"testing"
)

func TestParseGitDiff(t *testing.T) {
	diffText := `diff --git a/file1.txt b/file1.txt
index 1234567..890abcd 100644
--- a/file1.txt
+++ b/file1.txt
@@ -1,3 +1,3 @@
 line1
-line2
+line2_modified
 line3
diff --git a/newfile.txt b/newfile.txt
new file mode 100644
index 0000000..1234567
--- /dev/null
+++ b/newfile.txt
@@ -0,0 +1,2 @@
+new line 1
+new line 2
`

	files, err := ParseGitDiff(diffText)
	if err != nil {
		t.Fatalf("ParseGitDiff returned error: %v", err)
	}

	if len(files) != 2 {
		t.Errorf("Expected 2 files, got %d", len(files))
	}

	// Check first file
	if files[0].Filename != "file1.txt" {
		t.Errorf("Expected filename file1.txt, got %s", files[0].Filename)
	}
	if len(files[0].Hunks) != 1 {
		t.Errorf("Expected 1 hunk for file1.txt, got %d", len(files[0].Hunks))
	}

	hunk1 := files[0].Hunks[0]
	if len(hunk1.Lines) != 4 {
		t.Errorf("Expected 4 lines in hunk 1, got %d", len(hunk1.Lines))
	}

	// Check second file
	if files[1].Filename != "newfile.txt" {
		t.Errorf("Expected filename newfile.txt, got %s", files[1].Filename)
	}

	hunk2 := files[1].Hunks[0]
	if len(hunk2.Lines) != 2 {
		t.Errorf("Expected 2 lines in hunk 2, got %d", len(hunk2.Lines))
	}
	if hunk2.Lines[0].Type != LineAdded {
		t.Errorf("Expected LineAdded, got %v", hunk2.Lines[0].Type)
	}
}

func TestFormatForLLM(t *testing.T) {
	files := []FileDiff{
		{
			Filename: "test.txt",
			Hunks: []Hunk{
				{
					OldStart: 1, OldCount: 1,
					NewStart: 1, NewCount: 1,
					Lines: []Line{
						{Type: LineContext, Content: "context"},
						{Type: LineRemoved, Content: "removed"},
						{Type: LineAdded, Content: "added"},
					},
				},
			},
		},
	}

	output := FormatForLLM(files)
	expectedContains := []string{
		"## File: 'test.txt'",
		"__new hunk__",
		"__old hunk__",
		"+added",
		"-removed",
		"context",
	}

	for _, exp := range expectedContains {
		if !strings.Contains(output, exp) {
			t.Errorf("Expected output to contain '%s', but it didn't. Output:\n%s", exp, output)
		}
	}
}
