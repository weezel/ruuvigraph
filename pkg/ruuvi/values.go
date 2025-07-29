package ruuvi

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ReadAliases reads Ruuvitag aliases into memory for human friendly name mapping
func ReadAliases(filename string) (map[string]string, error) {
	file, err := os.OpenFile(filepath.Clean(filename), os.O_RDONLY, 0o600)
	if err != nil {
		return nil, fmt.Errorf("file open: %w", err)
	}
	defer file.Close()

	macNameMapping := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		splt := strings.Split(strings.TrimRight(scanner.Text(), "\r\t\n"), "|")
		if len(splt) != 2 {
			fmt.Printf("malformed line: %q\n", scanner.Text())
			continue
		}
		macNameMapping[splt[0]] = splt[1]
	}

	return macNameMapping, nil
}
