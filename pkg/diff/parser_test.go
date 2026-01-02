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

func TestParseGitDiff_EmptyDiff(t *testing.T) {
	files, err := ParseGitDiff("")
	if err != nil {
		t.Fatalf("ParseGitDiff returned error: %v", err)
	}
	if len(files) != 0 {
		t.Errorf("Expected 0 files for empty diff, got %d", len(files))
	}
}

func TestParseGitDiff_MultipleHunks(t *testing.T) {
	diffText := `diff --git a/file.txt b/file.txt
index 1234567..890abcd 100644
--- a/file.txt
+++ b/file.txt
@@ -1,3 +1,4 @@
 line1
+inserted
 line2
 line3
@@ -10,3 +11,3 @@
 line10
-line11
+line11_modified
 line12
`

	files, err := ParseGitDiff(diffText)
	if err != nil {
		t.Fatalf("ParseGitDiff returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	if len(files[0].Hunks) != 2 {
		t.Errorf("Expected 2 hunks, got %d", len(files[0].Hunks))
	}

	// Check first hunk
	hunk1 := files[0].Hunks[0]
	if hunk1.OldStart != 1 || hunk1.NewStart != 1 {
		t.Errorf("Hunk 1: expected OldStart=1, NewStart=1, got OldStart=%d, NewStart=%d", hunk1.OldStart, hunk1.NewStart)
	}

	// Check second hunk
	hunk2 := files[0].Hunks[1]
	if hunk2.OldStart != 10 || hunk2.NewStart != 11 {
		t.Errorf("Hunk 2: expected OldStart=10, NewStart=11, got OldStart=%d, NewStart=%d", hunk2.OldStart, hunk2.NewStart)
	}
}

func TestParseGitDiff_DeletedFile(t *testing.T) {
	diffText := `diff --git a/deleted.txt b/deleted.txt
deleted file mode 100644
index 1234567..0000000
--- a/deleted.txt
+++ /dev/null
@@ -1,3 +0,0 @@
-line1
-line2
-line3
`

	files, err := ParseGitDiff(diffText)
	if err != nil {
		t.Fatalf("ParseGitDiff returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	if files[0].Filename != "deleted.txt" {
		t.Errorf("Expected filename deleted.txt, got %s", files[0].Filename)
	}

	// All lines should be removed
	for _, line := range files[0].Hunks[0].Lines {
		if line.Type != LineRemoved {
			t.Errorf("Expected all lines to be LineRemoved, got %v", line.Type)
		}
	}
}

func TestParseGitDiff_BinaryFile(t *testing.T) {
	// Binary files should be parsed (even if they have no hunks)
	diffText := `diff --git a/image.png b/image.png
new file mode 100644
index 0000000..1234567
Binary files /dev/null and b/image.png differ
`

	files, err := ParseGitDiff(diffText)
	if err != nil {
		t.Fatalf("ParseGitDiff returned error: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("Expected 1 file, got %d", len(files))
	}

	if files[0].Filename != "image.png" {
		t.Errorf("Expected filename image.png, got %s", files[0].Filename)
	}

	// Binary files have no hunks
	if len(files[0].Hunks) != 0 {
		t.Errorf("Expected 0 hunks for binary file, got %d", len(files[0].Hunks))
	}
}

func TestCalculateLineNumbers(t *testing.T) {
	hunk := &Hunk{
		OldStart: 10,
		NewStart: 15,
		Lines: []Line{
			{Type: LineContext, Content: "context1"},
			{Type: LineRemoved, Content: "removed"},
			{Type: LineAdded, Content: "added"},
			{Type: LineContext, Content: "context2"},
		},
	}

	calculateLineNumbers(hunk)

	// Context line 1: old=10, new=15
	if hunk.Lines[0].OldNum != 10 || hunk.Lines[0].NewNum != 15 {
		t.Errorf("Line 0: expected old=10, new=15, got old=%d, new=%d", hunk.Lines[0].OldNum, hunk.Lines[0].NewNum)
	}

	// Removed line: old=11, new=0 (not set)
	if hunk.Lines[1].OldNum != 11 {
		t.Errorf("Line 1 (removed): expected old=11, got old=%d", hunk.Lines[1].OldNum)
	}

	// Added line: old=0 (not set), new=16
	if hunk.Lines[2].NewNum != 16 {
		t.Errorf("Line 2 (added): expected new=16, got new=%d", hunk.Lines[2].NewNum)
	}

	// Context line 2: old=12, new=17
	if hunk.Lines[3].OldNum != 12 || hunk.Lines[3].NewNum != 17 {
		t.Errorf("Line 3: expected old=12, new=17, got old=%d, new=%d", hunk.Lines[3].OldNum, hunk.Lines[3].NewNum)
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

func TestFormatForLLM_MultipleFiles(t *testing.T) {
	files := []FileDiff{
		{
			Filename: "file1.txt",
			Hunks: []Hunk{
				{
					OldStart: 1, OldCount: 1,
					NewStart: 1, NewCount: 1,
					Lines: []Line{
						{Type: LineAdded, Content: "new in file1"},
					},
				},
			},
		},
		{
			Filename: "file2.txt",
			Hunks: []Hunk{
				{
					OldStart: 1, OldCount: 1,
					NewStart: 1, NewCount: 1,
					Lines: []Line{
						{Type: LineRemoved, Content: "old in file2"},
					},
				},
			},
		},
	}

	output := FormatForLLM(files)

	if !strings.Contains(output, "## File: 'file1.txt'") {
		t.Errorf("Expected output to contain file1.txt header")
	}
	if !strings.Contains(output, "## File: 'file2.txt'") {
		t.Errorf("Expected output to contain file2.txt header")
	}
	if !strings.Contains(output, "+new in file1") {
		t.Errorf("Expected output to contain file1 content")
	}
	if !strings.Contains(output, "-old in file2") {
		t.Errorf("Expected output to contain file2 content")
	}
}

func TestFormatForLLM_EmptyFiles(t *testing.T) {
	files := []FileDiff{}
	output := FormatForLLM(files)
	if output != "" {
		t.Errorf("Expected empty output for empty files, got: %s", output)
	}
}

func TestFormatForLLM_LineNumbers(t *testing.T) {
	files := []FileDiff{
		{
			Filename: "test.txt",
			Hunks: []Hunk{
				{
					OldStart: 5, OldCount: 3,
					NewStart: 5, NewCount: 4,
					Lines: []Line{
						{Type: LineContext, Content: "context", NewNum: 5},
						{Type: LineAdded, Content: "added", NewNum: 6},
						{Type: LineContext, Content: "more", NewNum: 7},
					},
				},
			},
		},
	}

	output := FormatForLLM(files)

	// Check that line numbers are included
	if !strings.Contains(output, "5 ") {
		t.Errorf("Expected output to contain line number 5")
	}
}
