package FeltrinelliScraping

import (
	"encoding/json"
	"fmt"
	"strings"
)

func unmarshalJSON[T any](data []byte, target *T) error {
	err := json.Unmarshal(data, target)
	if err != nil {
		return fmt.Errorf("error unmarshaling JSON into %T: %w", target, err)
	}
	return nil
}

func getEANFromPath(path string) string {
	parts := strings.Split(path, "/")
	return parts[len(parts)-1]
}
