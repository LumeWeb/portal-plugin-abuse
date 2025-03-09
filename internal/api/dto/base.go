package dto

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	z "github.com/Oudwins/zog"
	"github.com/Oudwins/zog/parsers/zjson"
	"time"
)

type DTOValidator interface {
	Schema() *z.StructSchema
}

type BaseResponse struct {
	ID        uint      `json:"id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type BaseRequest struct {
	Description string `json:"description"`
}

func ToJSON[T any](data T) ([]byte, error) {
	return json.Marshal(data)
}

func Validate(validator DTOValidator) error {
	schema := validator.Schema()

	if schema == nil {
		return fmt.Errorf("schema is nil: invalid schema")
	}

	toJSON, err := ToJSON(validator)
	if err != nil {
		return err
	}

	errs := schema.Parse(zjson.Decode(bytes.NewReader(toJSON)), validator)
	if errs != nil {
		return err
	}

	errs = schema.Validate(validator)
	if errs != nil {
		// Use Zog's built-in sanitizer to convert errors to a simple map
		// This avoids any type assertions and uses the official API
		sanitized := z.Issues.SanitizeMap(errs)

		var valErrors error
		for _, messages := range sanitized {
			if len(messages) > 0 {
				valErrors = errors.Join(errors.New(messages[0]), valErrors)
			}
		}

		return valErrors
	}
	return nil
}
