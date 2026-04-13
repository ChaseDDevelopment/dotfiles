package executor

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"testing"
)

// tsLine matches the `[YYYY-MM-DD HH:MM:SS] ...` prefix so we can
// count complete log lines and reject torn writes.
var tsLine = regexp.MustCompile(
	`^\[\d{4}-\d{2}-\d{2} \d{2}:\d{2}:\d{2}\] `,
)

// TestLogFileTruncationRegression is a narrow regression test for
// commit cc17e19 ("Lock in fresh-per-run install.log with truncation
// regression test"). It pins the behavior that re-opening the same
// path via NewLogFile erases every prior line — not merely the first
// one. The existing happy-path in log_test.go writes one line; this
// test writes many and verifies none survive the second open.
func TestLogFileTruncationRegression(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "install.log")

	lf1, err := NewLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		lf1.Write("first-run-sentinel")
	}
	lf1.WriteRaw([]byte("raw-sentinel-1\nraw-sentinel-2\n"))
	if err := lf1.Close(); err != nil {
		t.Fatal(err)
	}

	lf2, err := NewLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	lf2.Write("fresh-line")
	if err := lf2.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if bytes.Contains(data, []byte("first-run-sentinel")) {
		t.Error(
			"first-run line leaked past truncation — regression",
		)
	}
	if bytes.Contains(data, []byte("raw-sentinel-1")) {
		t.Error(
			"raw-sentinel leaked past truncation — regression",
		)
	}
	if !bytes.Contains(data, []byte("fresh-line")) {
		t.Errorf(
			"fresh-line missing from new log: %q", content,
		)
	}
}

// TestLogFileHeavyConcurrentWrites stresses the mutex with 100
// goroutines writing distinct messages. The post-conditions: every
// line matches the timestamp regex (no torn lines), the total line
// count equals the expected sum (no dropped writes), and every
// per-goroutine identity is represented at least once (no lost
// messages).
func TestLogFileHeavyConcurrentWrites(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	lf, err := NewLogFile(path)
	if err != nil {
		t.Fatal(err)
	}

	const goroutines = 100
	const writesPer = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		id := i
		go func() {
			defer wg.Done()
			for j := 0; j < writesPer; j++ {
				// A unique, non-trivial message per write so we can
				// detect torn output (interleaving mid-line).
				lf.Write(
					"worker=" + itoa(id) + " iter=" + itoa(j),
				)
			}
		}()
	}
	wg.Wait()
	if err := lf.Close(); err != nil {
		t.Fatal(err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	// Parse every line; each must match the timestamp regex.
	scanner := bufio.NewScanner(bytes.NewReader(data))
	scanner.Buffer(make([]byte, 1<<16), 1<<20)
	var count int
	seen := make(map[string]bool)
	for scanner.Scan() {
		line := scanner.Text()
		if !tsLine.MatchString(line) {
			t.Fatalf(
				"torn line — missing timestamp prefix: %q",
				line,
			)
		}
		count++
		// Capture the worker id substring so we can verify every
		// goroutine produced at least one observable line.
		if idx := bytesIndex(
			[]byte(line), []byte("worker="),
		); idx >= 0 {
			seen[line[idx:]] = true
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}

	// Expected: 1 startup line + goroutines*writesPer.
	want := 1 + goroutines*writesPer
	if count != want {
		t.Errorf(
			"line count = %d, want %d (dropped or torn writes)",
			count, want,
		)
	}

	// Every goroutine's identity must have landed in at least one
	// line. A drop specific to one goroutine would fail this.
	presentWorkers := 0
	for i := 0; i < goroutines; i++ {
		for key := range seen {
			if bytesContains(
				[]byte(key),
				[]byte("worker="+itoa(i)+" "),
			) {
				presentWorkers++
				break
			}
		}
	}
	if presentWorkers != goroutines {
		t.Errorf(
			"only %d/%d goroutines visible in log — dropped writes",
			presentWorkers, goroutines,
		)
	}
}

// TestLogFileCloseThenWrite covers the post-close safety: after
// Close, Write and WriteRaw must be silent no-ops (not panics) and
// a second Close must return nil. The existing idempotent-close
// test doesn't interleave writes between closes.
func TestLogFileCloseThenWrite(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.log")
	lf, err := NewLogFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := lf.Close(); err != nil {
		t.Fatal(err)
	}
	// Any of these would panic if the mutex or file were unguarded
	// after Close.
	lf.Write("after-close-1")
	lf.Write("after-close-2")
	lf.WriteRaw([]byte("after-close-raw\n"))
	if err := lf.Close(); err != nil {
		t.Fatalf("second Close: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(data, []byte("after-close")) {
		t.Errorf(
			"post-close writes leaked into log: %q", data,
		)
	}
}

// itoa is a no-dep small positive int formatter — importing strconv
// would still be fine but this keeps the helper self-contained.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	return string(buf[i:])
}

func bytesIndex(haystack, needle []byte) int {
	return bytes.Index(haystack, needle)
}

func bytesContains(haystack, needle []byte) bool {
	return bytes.Contains(haystack, needle)
}
