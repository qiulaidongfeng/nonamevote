package data

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
)

type All[T any] []T

func (a *All[T]) Scan(value any) error {
	bytes, ok := value.([]byte)
	if !ok {
		return errors.New(fmt.Sprint("Failed to unmarshal JSONB value:", value))
	}

	err := json.Unmarshal(bytes, a)
	if err != nil {
		panic(string(bytes))
	}
	return err
}

// Value return json value, implement driver.Valuer interface
func (a All[T]) Value() (driver.Value, error) {
	return json.Marshal(a)
}
