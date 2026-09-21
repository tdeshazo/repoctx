// Package benchfixture creates the synthetic repository used by performance tests.
package benchfixture

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write creates files one-per-directory and returns their total source bytes.
func Write(root string, files int) (int, error) {
	module := []byte("module example.test/profile\n\ngo 1.23\n")
	if err := os.WriteFile(filepath.Join(root, "go.mod"), module, 0o600); err != nil {
		return 0, err
	}
	sourceBytes := 0
	for i := 0; i < files; i++ {
		dir := filepath.Join(root, fmt.Sprintf("pkg%04d", i))
		if err := os.Mkdir(dir, 0o700); err != nil {
			return 0, err
		}
		source := []byte(fmt.Sprintf(`package pkg%04d

type Record struct { Value int }

func Compute(value int) int { return helper(value) + 1 }

func helper(value int) int { return value * 2 }
`, i))
		if err := os.WriteFile(filepath.Join(dir, "value.go"), source, 0o600); err != nil {
			return 0, err
		}
		sourceBytes += len(source)
	}
	return sourceBytes, nil
}
