package data

import (
	"encoding/csv"
	"fmt"
	"os"
)

// WriteCSV writes all rows to given file path.
func WriteCSV(file string, rows [][]string) error {
	// generate output csv file.
	f, err := os.Create(file)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %v", file, err)
	}

	// write the designated objects.
	w := csv.NewWriter(f)
	if err := w.WriteAll(rows); err != nil {
		return fmt.Errorf("failed to write csv: %v", err)
	}

	return nil
}

// FormatBool converts a boolean to string "0" or "1".
func FormatBool(b bool) string {
	if b {
		return "1"
	}
	return "0"
}
