// Copyright 2023 Greenmask
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transformers

import (
	"strings"

	"github.com/greenmaskio/greenmask/internal/domains"
	"github.com/greenmaskio/greenmask/pkg/toolkit"
	"github.com/pkg/errors"
)

// GetDefaultTransformerForColumn returns a default transformer configuration
// for a column based on its PostgreSQL data type. Returns nil if no suitable
// default transformer is available for the column type.
func GetDefaultTransformerForColumn(column *toolkit.Column) (*domains.TransformerConfig, error) {
	typeName, _ := column.GetType()
	canonicalType := column.CanonicalTypeName
	if canonicalType != "" {
		typeName = canonicalType
	}

	// Handle array types by checking for [] suffix or _ prefix
	if strings.HasSuffix(typeName, "[]") || strings.HasPrefix(typeName, "_") {
		return getDefaultTransformerForArrayType(column, typeName)
	}

	return getDefaultTransformerForScalarType(column, typeName)
}

// getDefaultTransformerForScalarType returns default transformer for scalar types
func getDefaultTransformerForScalarType(column *toolkit.Column, typeName string) (*domains.TransformerConfig, error) {
	switch strings.ToLower(typeName) {
	// Text types
	case "text", "varchar", "character varying", "char", "character", "bpchar":
		return &domains.TransformerConfig{
			Name: "RandomString",
			Params: toolkit.StaticParameters{
				"column":     toolkit.ParamsValue(column.Name),
				"min_length": toolkit.ParamsValue("5"),
				"max_length": toolkit.ParamsValue("20"),
			},
		}, nil

	// Integer types
	case "integer", "int", "int4", "bigint", "int8", "smallint", "int2":
		return &domains.TransformerConfig{
			Name: "RandomInt",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"min":    toolkit.ParamsValue("1"),
				"max":    toolkit.ParamsValue("2147483647"),
			},
		}, nil

	// Numeric/decimal types
	case "numeric", "decimal":
		return &domains.TransformerConfig{
			Name: "RandomNumeric",
			Params: toolkit.StaticParameters{
				"column":    toolkit.ParamsValue(column.Name),
				"min":       toolkit.ParamsValue("1"),
				"max":       toolkit.ParamsValue("999999"),
				"precision": toolkit.ParamsValue("10"),
				"scale":     toolkit.ParamsValue("2"),
			},
		}, nil

	// Float types
	case "real", "float4", "double precision", "float8":
		return &domains.TransformerConfig{
			Name: "RandomFloat",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"min":    toolkit.ParamsValue("1.0"),
				"max":    toolkit.ParamsValue("1000000.0"),
			},
		}, nil

	// Date/time types - different formats based on type
	case "date":
		return &domains.TransformerConfig{
			Name: "RandomDate",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"min":    toolkit.ParamsValue("1970-01-01"),
				"max":    toolkit.ParamsValue("2024-12-31"),
			},
		}, nil

	case "timestamp", "timestamp without time zone":
		return &domains.TransformerConfig{
			Name: "RandomDate",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"min":    toolkit.ParamsValue("1970-01-01 00:00:00"),
				"max":    toolkit.ParamsValue("2024-12-31 23:59:59"),
			},
		}, nil

	case "timestamptz", "timestamp with time zone":
		return &domains.TransformerConfig{
			Name: "RandomDate",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"min":    toolkit.ParamsValue("1970-01-01 00:00:00+00"),
				"max":    toolkit.ParamsValue("2024-12-31 23:59:59+00"),
			},
		}, nil

	// Boolean type
	case "boolean", "bool":
		return &domains.TransformerConfig{
			Name: "RandomBool",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
			},
		}, nil

	// UUID type
	case "uuid":
		return &domains.TransformerConfig{
			Name: "RandomUuid",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
			},
		}, nil

	// JSON types
	case "json", "jsonb":
		return &domains.TransformerConfig{
			Name: "Replace",
			Params: toolkit.StaticParameters{
				"column": toolkit.ParamsValue(column.Name),
				"value":  toolkit.ParamsValue(`{}`),
			},
		}, nil

	// For unsupported types, return nil (no transformation)
	default:
		return nil, errors.Errorf("unable to get default transformer for column %s and type %s", column.Name, typeName)
	}
}

// getDefaultTransformerForArrayType returns default transformer for array types
func getDefaultTransformerForArrayType(column *toolkit.Column, typeName string) (*domains.TransformerConfig, error) {
	// For array types, we will replace the value with an empty array
	return &domains.TransformerConfig{
		Name: "Replace",
		Params: toolkit.StaticParameters{
			"column":    toolkit.ParamsValue(column.Name),
			"value":     toolkit.ParamsValue(`{}`),
			"keep_null": toolkit.ParamsValue("true"),
		},
	}, nil
}
