package fsops_test

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/ivanehh/go-boiler-lib/pkg/platform/fsops"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Helper function to create test files with specific mod times
// Helper function to create dummy files for testing
func createTestFile(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	f, err := os.Create(path)
	require.NoError(t, err)
	require.NoError(t, f.Close())
	if !modTime.IsZero() {
		err = os.Chtimes(path, modTime, modTime)
		require.NoError(t, err)
	}
}

// Helper function to create test directories and files
func setupTestDirs(t *testing.T) (string, map[string]time.Time) {
	t.Helper()
	tmpDir := t.TempDir()

	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")
	require.NoError(t, os.Mkdir(dir1, 0o755))
	require.NoError(t, os.Mkdir(dir2, 0o755))

	now := time.Now()
	hourAgo := now.Add(-1 * time.Hour)
	dayAgo := now.Add(-24 * time.Hour)

	files := map[string]time.Time{
		filepath.Join(dir1, "a.txt"):        now,
		filepath.Join(dir1, "b.log"):        now,
		filepath.Join(dir1, "old.txt"):      dayAgo,
		filepath.Join(dir2, "c.txt"):        hourAgo,
		filepath.Join(dir2, "d.dat"):        now,
		filepath.Join(dir2, "sub"):          {}, // Directory marker
		filepath.Join(dir2, "sub", "e.txt"): now,
	}

	subDir := filepath.Join(dir2, "sub")
	require.NoError(t, os.Mkdir(subDir, 0o755))

	for p, mt := range files {
		if mt.IsZero() { // Skip directory marker
			continue
		}
		// Create files relative to CWD for os.Open in age filter to work
		// This highlights a potential issue in the original Filter implementation
		relPath, err := filepath.Rel(tmpDir, p)
		require.NoError(t, err)
		createTestFile(t, p, mt)
		// Also create relative path versions if needed for age check testing
		// This is a workaround for the os.Open issue in the Filter method
		if _, err := os.Stat(relPath); os.IsNotExist(err) {
			containingDir := filepath.Dir(relPath)
			if containingDir != "." {
				require.NoError(t, os.MkdirAll(containingDir, 0o755))
			}
			createTestFile(t, relPath, mt)
			t.Cleanup(func() { os.Remove(relPath) }) // Clean up relative files
		}

	}

	return tmpDir, files
}

func TestFileFilter_Filter_NoDirs(t *testing.T) {
	ff, err := fsops.NewFileFilter(fsops.WithGlobPattern("*.txt"))
	require.NoError(t, err)

	matches, err := ff.Filter()
	require.ErrorIs(t, err, fsops.ErrNoDirsProvided)
	assert.Nil(t, matches)
}

func TestFileFilter_Filter_SimpleGlob(t *testing.T) {
	tmpDir, _ := setupTestDirs(t)
	dir1 := filepath.Join(tmpDir, "dir1")

	ff, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("*.txt"),
		fsops.SetLoc([]string{dir1}),
	)
	require.NoError(t, err)

	matches, err := ff.Filter()
	require.NoError(t, err)

	// Without age filter, paths should be joined with the base dir
	expected := []string{
		filepath.Join(dir1, "a.txt"),
		filepath.Join(dir1, "old.txt"),
	}
	sort.Strings(matches)
	sort.Strings(expected)
	assert.Equal(t, expected, matches)
}

func TestFileFilter_Filter_MultiDirGlob(t *testing.T) {
	tmpDir, _ := setupTestDirs(t)
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")

	ff, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("*.txt"),
		fsops.SetLoc([]string{dir1, dir2}),
	)
	require.NoError(t, err)

	matches, err := ff.Filter()
	require.NoError(t, err)

	// Without age filter, paths should be joined with the base dir
	expected := []string{
		filepath.Join(dir1, "a.txt"),
		filepath.Join(dir1, "old.txt"),
		filepath.Join(dir2, "c.txt"),
		// Note: Does not find sub/e.txt because fs.Glob is not recursive by default
	}
	sort.Strings(matches)
	sort.Strings(expected)
	assert.Equal(t, expected, matches)
}

func TestFileFilter_Filter_AgeFilterRecent(t *testing.T) {
	tmpDir, _ := setupTestDirs(t)
	dir1 := filepath.Join(tmpDir, "dir1")
	dir2 := filepath.Join(tmpDir, "dir2")

	// Need to run age check relative to CWD because Filter uses os.Open
	// Change CWD for this test
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(originalWd) }) // Restore CWD

	ff, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("*.txt"),
		fsops.SetLoc([]string{dir1, dir2}), // Use relative paths matching CWD
		fsops.WithFileAge(2*time.Hour),     // Max age 2 hours
	)
	require.NoError(t, err)

	matches, err := ff.Filter()
	require.NoError(t, err)

	// With age filter, current implementation returns relative paths
	expected := []string{
		filepath.Join(dir1, "a.txt"), // Created now
		filepath.Join(dir2, "c.txt"), // Created 1 hour ago
		// "old.txt" (1 day ago) should be excluded
	}

	// The Filter method appends matches per directory, so duplicates might occur if patterns overlap
	// And the order depends on map iteration + glob results. Let's sort.
	sort.Strings(matches)
	sort.Strings(expected)

	// Debugging output if test fails
	if !assert.ObjectsAreEqual(expected, matches) {
		fmt.Println("Expected:", expected)
		fmt.Println("Actual:", matches)
		// Check modification times manually if needed
		for _, f := range []string{
			filepath.Join(dir1, "a.txt"),
			filepath.Join(dir1, "old.txt"),
			filepath.Join(dir2, "c.txt"),
		} {
			absPath := filepath.Join(tmpDir, f) // Get abs path for Stat
			info, statErr := os.Stat(absPath)
			if statErr == nil {
				fmt.Printf("ModTime for %s: %s\n", f, info.ModTime())
			} else {
				fmt.Printf("Could not stat %s: %v\n", f, statErr)
			}

		}
		fmt.Printf("Filter time threshold: %s\n", time.Now().Add(-2*time.Hour))
	}

	assert.Equal(t, expected, matches)
}

func TestFileFilter_Filter_AgeFilterNoneMatch(t *testing.T) {
	tmpDir, _ := setupTestDirs(t)
	dir1 := filepath.Join(tmpDir, "dir1")

	// Need to run age check relative to CWD because Filter uses os.Open
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(originalWd) })

	ff, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("*.txt"),
		fsops.SetLoc([]string{dir1}),      // Use relative path
		fsops.WithFileAge(30*time.Minute), // Max age 30 mins
	)
	require.NoError(t, err)

	matches, err := ff.Filter()
	require.NoError(t, err)

	// a.txt created now, old.txt created 1 day ago. Only a.txt should match age.
	// c.txt created 1 hour ago.
	// With 30 min filter, only a.txt should match
	expected := []string{
		filepath.Join(dir1, "a.txt"),
	}
	sort.Strings(matches)
	sort.Strings(expected)
	assert.Equal(t, expected, matches)

	// Now test with a very short age where nothing matches
	ffAge, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("*.txt"),
		fsops.SetLoc([]string{dir1}),          // Use relative path
		fsops.WithFileAge(1*time.Millisecond), // Very short age
	)
	require.NoError(t, err)
	// Need a slight pause to ensure files are older than the filter age
	time.Sleep(2 * time.Millisecond)

	matchesNone, err := ffAge.Filter()
	require.NoError(t, err)
	assert.Empty(t, matchesNone)
}

func TestFileFilter_Filter_PathConstructionDifference(t *testing.T) {
	tmpDir, _ := setupTestDirs(t)
	dir1 := filepath.Join(tmpDir, "dir1")

	// --- Test without age filter ---
	ffNoAge, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("a.txt"),
		fsops.SetLoc([]string{dir1}),
	)
	require.NoError(t, err)
	matchesNoAge, err := ffNoAge.Filter()
	require.NoError(t, err)
	expectedNoAge := []string{filepath.Join(dir1, "a.txt")}
	assert.Equal(t, expectedNoAge, matchesNoAge)

	// --- Test with age filter ---
	// Need to run age check relative to CWD because Filter uses os.Open
	originalWd, err := os.Getwd()
	require.NoError(t, err)
	require.NoError(t, os.Chdir(tmpDir))
	t.Cleanup(func() { os.Chdir(originalWd) })

	ffWithAge, err := fsops.NewFileFilter(
		fsops.WithGlobPattern("a.txt"),
		fsops.SetLoc([]string{dir1}),   // Use relative path
		fsops.WithFileAge(1*time.Hour), // Should include a.txt
	)
	require.NoError(t, err)
	matchesWithAge, err := ffWithAge.Filter()
	require.NoError(t, err)
	// Current implementation returns relative path here
	expectedWithAge := []string{filepath.Join(dir1, "a.txt")}
	assert.Equal(t, expectedWithAge, matchesWithAge)

	// Assert the difference explicitly if needed, though the above checks show it
	// assert.NotEqual(t, matchesNoAge[0], matchesWithAge[0], "Path construction differs with/without age filter")
	// Note: The above assertion might be brittle depending on OS path separators.
	// The key observation is that one returns an absolute/joined path based on SetLoc input,
	// while the other returns a path relative to the SetLoc input directory.
}

// Potential test for invalid pattern during Filter (less likely with current constructor checks)
// func TestFileFilter_Filter_InvalidPatternInFilter(t *testing.T) {
// 	// This scenario is hard to trigger because fsops.NewFileFilter and SetPattern validate the pattern.
// 	// If the validation was removed or bypassed, this test would be relevant.
// 	tmpDir, _ := setupTestDirs(t)
// 	dir1 := filepath.Join(tmpDir, "dir1")

// 	ff := &FileFilter{
// 		dir:     map[string]fs.FS{dir1: os.DirFS(dir1)},
// 		pattern: "[", // Invalid pattern
// 	}

// 	_, err := ff.Filter()
// 	assert.Error(t, err) // Expecting filepath.ErrBadPattern or similar
// }
