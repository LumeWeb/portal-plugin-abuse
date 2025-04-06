package dto

import (
	"github.com/Oudwins/zog"
	"github.com/Oudwins/zog/conf"
)

// StringLike creates a StringSchema for type aliases of string.
func StringLike[T ~string](opts ...zog.SchemaOption) *zog.StringSchema[T] {
	s := &zog.StringSchema[T]{}

	// Custom coercer to handle the type alias conversion during coercion.
	customCoercer := func(data any) (any, error) {
		str, err := conf.Coercers.String(data) // Use the default string coercer first
		if err != nil {
			return nil, err
		}

		// Convert to the type alias.
		converted := T(str.(string)) // Direct conversion to type alias
		return converted, nil
	}

	opts = append(opts, zog.WithCoercer(customCoercer)) // Set the custom coercer using the With option

	for _, opt := range opts {
		opt(s)
	}
	return s
}
