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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/greenmaskio/greenmask/pkg/toolkit"
)

func TestGetDefaultTransformerForColumn(t *testing.T) {
	tests := []struct {
		name         string
		columnName   string
		typeName     string
		expectedName string
		shouldBeNil  bool
		shouldErr    bool
	}{
		// Text types
		{
			name:         "text column",
			columnName:   "description",
			typeName:     "text",
			expectedName: "RandomString",
		},
		{
			name:         "varchar column",
			columnName:   "name",
			typeName:     "varchar",
			expectedName: "RandomString",
		},
		{
			name:         "character varying column",
			columnName:   "title",
			typeName:     "character varying",
			expectedName: "RandomString",
		},

		// Integer types
		{
			name:         "integer column",
			columnName:   "age",
			typeName:     "integer",
			expectedName: "RandomInt",
		},
		{
			name:         "bigint column",
			columnName:   "id",
			typeName:     "bigint",
			expectedName: "RandomInt",
		},
		{
			name:         "smallint column",
			columnName:   "count",
			typeName:     "smallint",
			expectedName: "RandomInt",
		},

		// Numeric types
		{
			name:         "numeric column",
			columnName:   "price",
			typeName:     "numeric",
			expectedName: "RandomNumeric",
		},
		{
			name:         "decimal column",
			columnName:   "amount",
			typeName:     "decimal",
			expectedName: "RandomNumeric",
		},

		// Float types
		{
			name:         "real column",
			columnName:   "rating",
			typeName:     "real",
			expectedName: "RandomFloat",
		},
		{
			name:         "double precision column",
			columnName:   "score",
			typeName:     "double precision",
			expectedName: "RandomFloat",
		},

		// Date/time types
		{
			name:         "date column",
			columnName:   "birth_date",
			typeName:     "date",
			expectedName: "RandomDate",
		},
		{
			name:         "timestamp column",
			columnName:   "created_at",
			typeName:     "timestamp",
			expectedName: "RandomDate",
		},
		{
			name:         "timestamptz column",
			columnName:   "updated_at",
			typeName:     "timestamptz",
			expectedName: "RandomDate",
		},

		// Boolean type
		{
			name:         "boolean column",
			columnName:   "is_active",
			typeName:     "boolean",
			expectedName: "RandomBool",
		},

		// UUID type
		{
			name:         "uuid column",
			columnName:   "user_id",
			typeName:     "uuid",
			expectedName: "RandomUuid",
		},

		// JSON types
		{
			name:         "json column",
			columnName:   "metadata",
			typeName:     "json",
			expectedName: "Replace",
		},
		{
			name:         "jsonb column",
			columnName:   "data",
			typeName:     "jsonb",
			expectedName: "Replace",
		},

		// Unsupported types should return nil
		{
			name:        "unsupported type",
			columnName:  "custom_data",
			typeName:    "custom_type",
			shouldBeNil: true,
			shouldErr:   true,
		},
		{
			name:        "geography type",
			columnName:  "location",
			typeName:    "geography",
			shouldBeNil: true,
			shouldErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := &toolkit.Column{
				Name:     tt.columnName,
				TypeName: tt.typeName,
			}

			result, err := GetDefaultTransformerForColumn(column)

			if tt.shouldErr {
				assert.Error(t, err, "Expected an error for unsupported type")
			} else {
				assert.NoError(t, err, "Expected no error for supported type")
			}

			if tt.shouldBeNil {
				assert.Nil(t, result, "Expected nil transformer for unsupported type")
			} else {
				require.NotNil(t, result, "Expected non-nil transformer for supported type")
				assert.Equal(t, tt.expectedName, result.Name, "Transformer name should match expected")

				// Check that column parameter is set correctly
				columnParam, exists := result.Params["column"]
				require.True(t, exists, "Column parameter should exist")
				assert.Equal(t, tt.columnName, string(columnParam), "Column parameter should match column name")

				// For RandomString transformer, check that min_length and max_length are set
				if result.Name == "RandomString" {
					minLengthParam, exists := result.Params["min_length"]
					require.True(t, exists, "min_length parameter should exist for RandomString")
					assert.Equal(t, "5", string(minLengthParam), "min_length should be 5")

					maxLengthParam, exists := result.Params["max_length"]
					require.True(t, exists, "max_length parameter should exist for RandomString")
					assert.Equal(t, "20", string(maxLengthParam), "max_length should be 20")
				}

				// For Replace transformer (used for JSON), check that value parameter is set
				if result.Name == "Replace" {
					valueParam, exists := result.Params["value"]
					require.True(t, exists, "value parameter should exist for Replace")
					assert.Equal(t, "{}", string(valueParam), "value should be empty JSON object")
				}
			}
		})
	}
}

func TestGetDefaultTransformerForColumn_CanonicalTypeName(t *testing.T) {
	// Test that canonical type name takes precedence over type name
	column := &toolkit.Column{
		Name:              "test_col",
		TypeName:          "some_alias",
		CanonicalTypeName: "text",
	}

	result, err := GetDefaultTransformerForColumn(column)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, "RandomString", result.Name, "Should use canonical type name")
}

func TestGetDefaultTransformerForColumn_ArrayTypes(t *testing.T) {
	tests := []struct {
		name         string
		typeName     string
		expectedName string
	}{
		{
			name:         "text array",
			typeName:     "text[]",
			expectedName: "Replace",
		},
		{
			name:         "text array with underscore",
			typeName:     "_text",
			expectedName: "Replace",
		},
		{
			name:         "integer array",
			typeName:     "integer[]",
			expectedName: "Replace",
		},
		{
			name:         "integer array with underscore",
			typeName:     "_int4",
			expectedName: "Replace",
		},
		{
			name:         "boolean array",
			typeName:     "boolean[]",
			expectedName: "Replace",
		},
		{
			name:         "uuid array",
			typeName:     "uuid[]",
			expectedName: "Replace",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := &toolkit.Column{
				Name:     "test_array",
				TypeName: tt.typeName,
			}

			result, err := GetDefaultTransformerForColumn(column)
			require.NoError(t, err)
			require.NotNil(t, result, "Array type should have default transformer")
			assert.Equal(t, tt.expectedName, result.Name, "Array should use base type transformer")
		})
	}
}

func TestGetDefaultTransformerForColumn_CaseInsensitive(t *testing.T) {
	tests := []struct {
		typeName     string
		expectedName string
	}{
		{"TEXT", "RandomString"},
		{"INTEGER", "RandomInt"},
		{"BOOLEAN", "RandomBool"},
		{"UUID", "RandomUuid"},
	}

	for _, tt := range tests {
		t.Run(tt.typeName, func(t *testing.T) {
			column := &toolkit.Column{
				Name:     "test_col",
				TypeName: tt.typeName,
			}

			result, err := GetDefaultTransformerForColumn(column)
			require.NoError(t, err)
			require.NotNil(t, result, "Case should not matter")
			assert.Equal(t, tt.expectedName, result.Name)
		})
	}
}

func TestGetDefaultTransformerForColumn_DateTimeFormats(t *testing.T) {
	tests := []struct {
		name        string
		typeName    string
		expectedMin string
		expectedMax string
	}{
		{
			name:        "date type",
			typeName:    "date",
			expectedMin: "1970-01-01",
			expectedMax: "2024-12-31",
		},
		{
			name:        "timestamp type",
			typeName:    "timestamp",
			expectedMin: "1970-01-01 00:00:00",
			expectedMax: "2024-12-31 23:59:59",
		},
		{
			name:        "timestamp without time zone",
			typeName:    "timestamp without time zone",
			expectedMin: "1970-01-01 00:00:00",
			expectedMax: "2024-12-31 23:59:59",
		},
		{
			name:        "timestamptz type",
			typeName:    "timestamptz",
			expectedMin: "1970-01-01 00:00:00+00",
			expectedMax: "2024-12-31 23:59:59+00",
		},
		{
			name:        "timestamp with time zone",
			typeName:    "timestamp with time zone",
			expectedMin: "1970-01-01 00:00:00+00",
			expectedMax: "2024-12-31 23:59:59+00",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			column := &toolkit.Column{
				Name:     "test_col",
				TypeName: tt.typeName,
			}

			result, err := GetDefaultTransformerForColumn(column)
			require.NoError(t, err)
			require.NotNil(t, result, "Date/time types should have default transformer")
			assert.Equal(t, "RandomDate", result.Name, "Should use RandomDate transformer")

			// Check min and max parameters have correct format for the column type
			minParam, exists := result.Params["min"]
			require.True(t, exists, "Min parameter should exist")
			assert.Equal(t, tt.expectedMin, string(minParam), "Min parameter format should match column type")

			maxParam, exists := result.Params["max"]
			require.True(t, exists, "Max parameter should exist")
			assert.Equal(t, tt.expectedMax, string(maxParam), "Max parameter format should match column type")
		})
	}
}
