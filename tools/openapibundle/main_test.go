package main

import (
	"context"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/stretchr/testify/require"
)

func fixture(name string) string { return filepath.Join("testdata", name, "openapi.yaml") }

func TestBundle_RewritesRefsToRootKeys(t *testing.T) {
	out, err := bundle(fixture("ok"))
	require.NoError(t, err)
	require.NotContains(t, string(out), ".yaml#", "no external refs may remain")

	doc, err := openapi3.NewLoader().LoadFromData(out) // external refs not allowed
	require.NoError(t, err)
	require.NoError(t, doc.Validate(context.Background()))

	var names []string
	for name, s := range doc.Components.Schemas {
		require.Empty(t, s.Ref, "component %s must be inlined", name)
		names = append(names, name)
	}
	sort.Strings(names)
	require.Equal(t, []string{"Item", "ItemResponse", "Problem"}, names)

	props := doc.Components.Schemas["ItemResponse"].Value.Properties
	require.Equal(t, "#/components/schemas/Item", props["data"].Ref, "same-file ref #/Thing")
	require.Equal(t, "#/components/schemas/Problem", props["error"].Ref, "cross-file ref ./common.yaml#/Problem")

	op := doc.Paths.Find("/things").Get
	require.Equal(t, "#/components/schemas/ItemResponse", op.Responses.Status(200).Value.Content.Get("application/json").Schema.Ref)
	require.Equal(t, "#/components/schemas/Problem", op.Responses.Status(500).Value.Content.Get("application/json").Schema.Ref)
}

func TestBundle_IsDeterministic(t *testing.T) {
	tests := []string{fixture("ok"), filepath.Join("..", "..", "api", "openapi.yaml")}
	for _, in := range tests {
		t.Run(in, func(t *testing.T) {
			first, err := bundle(in)
			require.NoError(t, err)
			second, err := bundle(in)
			require.NoError(t, err)
			require.Equal(t, string(first), string(second))
		})
	}
}

func TestBundle_Errors(t *testing.T) {
	tests := []struct {
		name    string
		wantErr []string // substrings, each must appear exactly once
	}{
		{"missing_ref", []string{"refs not listed in root components.schemas", "schemas schemas/thing.yaml#/Other"}},
		{"external_parameter", []string{"refs not listed in root components.schemas", "parameters parameters.yaml#/ID"}},
		{"external_response", []string{"refs not listed in root components.schemas", "responses responses.yaml#/Ok"}},
		{"duplicate_target", []string{"components.schemas.A and components.schemas.B point at the same schema schemas/thing.yaml#/Thing"}},
		{"inline_component", []string{"components.schemas.Thing must be a $ref"}},
		{"unlisted_schema_key", []string{"no root components.schemas entry", "schemas/thing.yaml#/Orphan"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, err := bundle(fixture(tc.name))
			require.Error(t, err)
			require.Nil(t, out)
			t.Log(err)
			for _, want := range tc.wantErr {
				require.Equal(t, 1, strings.Count(err.Error(), want), "want %q once in: %v", want, err)
			}
		})
	}
}
