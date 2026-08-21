// Copyright 2020 Google LLC. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

var openapiTests = []struct {
	name      string
	path      string
	protofile string
}{
	{name: "Google Library example", path: "examples/google/example/library/v1/", protofile: "library.proto"},
	{name: "Body mapping", path: "examples/tests/bodymapping/", protofile: "message.proto"},
	{name: "Map fields", path: "examples/tests/mapfields/", protofile: "message.proto"},
	{name: "Path params", path: "examples/tests/pathparams/", protofile: "message.proto"},
	{name: "Protobuf types", path: "examples/tests/protobuftypes/", protofile: "message.proto"},
	{name: "RPC types", path: "examples/tests/rpctypes/", protofile: "message.proto"},
	{name: "JSON options", path: "examples/tests/jsonoptions/", protofile: "message.proto"},
	{name: "Ignore services without annotations", path: "examples/tests/noannotations/", protofile: "message.proto"},
	{name: "Enum Options", path: "examples/tests/enumoptions/", protofile: "message.proto"},
	{name: "OpenAPIv3 Annotations", path: "examples/tests/openapiv3annotations/", protofile: "message.proto"},
	{name: "AllOf Wrap Message", path: "examples/tests/allofwrap/", protofile: "message.proto"},
	{name: "Additional Bindings", path: "examples/tests/additional_bindings/", protofile: "message.proto"},
}

// Set this to true to generate/overwrite the fixtures. Make sure you set it back
// to false before you commit it.
const GENERATE_FIXTURES = false

const TEMP_FILE = "openapi.yaml"

func CopyFixture(result, fixture string) error {
	in, err := os.Open(result)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(fixture)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}

func TestGenerateFixturesIsFalse(t *testing.T) {
	// This is here to ensure the PR builds fail if someone
	// accidentally commits GENERATE_FIXTURES = true
	if GENERATE_FIXTURES {
		t.Fatalf("GENERATE_FIXTURES is true")
	}
}

func TestOpenAPIProtobufNaming(t *testing.T) {
	for _, tt := range openapiTests {
		fixture := path.Join(tt.path, "openapi.yaml")
		if _, err := os.Stat(fixture); errors.Is(err, os.ErrNotExist) {
			if !GENERATE_FIXTURES {
				continue
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			// Run protoc and the protoc-gen-openapi plugin to generate an OpenAPI spec.
			err := exec.Command("protoc",
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(tt.path, tt.protofile),
				"--openapi_out=naming=proto:.").Run()
			if err != nil {
				t.Fatalf("protoc failed: %+v", err)
			}
			if GENERATE_FIXTURES {
				err := CopyFixture(TEMP_FILE, fixture)
				if err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else {
				// Verify that the generated spec matches our expected version.
				err = exec.Command("diff", TEMP_FILE, fixture).Run()
				if err != nil {
					t.Fatalf("Diff failed: %+v", err)
				}
			}
			// if the test succeeded, clean up
			os.Remove(TEMP_FILE)
		})
	}
}

func TestOpenAPIFQSchemaNaming(t *testing.T) {
	// create temp directory for source_relative outputs
	tempDir := "tmp"
	if err := os.MkdirAll(path.Join(tempDir, "examples"), os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)
	// run protoc with source_relative options on all examples
	args := []string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		fmt.Sprintf("--openapi_out=fq_schema_naming=1:%s/examples", tempDir),
		"--openapi_opt=output_mode=source_relative",
	}
	for _, tt := range openapiTests {
		args = append(args, path.Join(tt.path, tt.protofile))
	}
	if err := exec.Command("protoc", args...).Run(); err != nil {
		t.Fatalf("protoc %v failed: %+v", strings.Join(args, " "), err)
	}

	for _, tt := range openapiTests {
		fixture := path.Join(tt.path, "openapi_fq_schema_naming.yaml")
		if _, err := os.Stat(fixture); errors.Is(err, os.ErrNotExist) {
			if !GENERATE_FIXTURES {
				continue
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			// Run protoc and the protoc-gen-openapi plugin to generate an OpenAPI spec.
			err := exec.Command("protoc",
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(tt.path, tt.protofile),
				"--openapi_out=fq_schema_naming=1:.").Run()
			if err != nil {
				t.Fatalf("protoc failed: %+v", err)
			}
			if GENERATE_FIXTURES {
				err := CopyFixture(TEMP_FILE, fixture)
				if err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else {
				// Verify that the generated spec matches our expected version.
				err = exec.Command("diff", TEMP_FILE, fixture).Run()
				if err != nil {
					t.Fatalf("Diff failed: %+v", err)
				}
				// Verify that the generated spec matches the source_relative version
				sourceRelativeFile := strings.TrimSuffix(tt.protofile, filepath.Ext(tt.protofile)) + ".openapi.yaml"
				sourceRelativeOut := path.Join(tempDir, tt.path, sourceRelativeFile)
				err = exec.Command("diff", sourceRelativeOut, fixture).Run()
				if err != nil {
					t.Fatalf("Diff %v %v: %+v", sourceRelativeOut, fixture, err)
				}
			}
			// if the test succeeded, clean up
			os.Remove(TEMP_FILE)
		})
	}
}

func TestOpenAPIJSONNaming(t *testing.T) {
	for _, tt := range openapiTests {
		fixture := path.Join(tt.path, "openapi_json.yaml")
		if _, err := os.Stat(fixture); errors.Is(err, os.ErrNotExist) {
			if !GENERATE_FIXTURES {
				continue
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			// Run protoc and the protoc-gen-openapi plugin to generate an OpenAPI spec with JSON naming.
			err := exec.Command("protoc",
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(tt.path, tt.protofile),
				"--openapi_out=version=1.2.3:.").Run()
			if err != nil {
				t.Fatalf("protoc failed: %+v", err)
			}
			if GENERATE_FIXTURES {
				err := CopyFixture(TEMP_FILE, fixture)
				if err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else {
				// Verify that the generated spec matches our expected version.
				err = exec.Command("diff", TEMP_FILE, fixture).Run()
				if err != nil {
					t.Fatalf("Diff failed: %+v", err)
				}
			}
			// if the test succeeded, clean up
			os.Remove(TEMP_FILE)
		})
	}
}

func TestOpenAPIStringEnums(t *testing.T) {
	for _, tt := range openapiTests {
		fixture := path.Join(tt.path, "openapi_string_enum.yaml")
		if _, err := os.Stat(fixture); errors.Is(err, os.ErrNotExist) {
			if !GENERATE_FIXTURES {
				continue
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			// Run protoc and the protoc-gen-openapi plugin to generate an OpenAPI spec with string Enums.
			err := exec.Command("protoc",
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(tt.path, tt.protofile),
				"--openapi_out=enum_type=string:.").Run()
			if err != nil {
				t.Fatalf("protoc failed: %+v", err)
			}
			if GENERATE_FIXTURES {
				err := CopyFixture(TEMP_FILE, fixture)
				if err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else {
				// Verify that the generated spec matches our expected version.
				err = exec.Command("diff", TEMP_FILE, fixture).Run()
				if err != nil {
					t.Fatalf("diff failed: %+v", err)
				}
			}
			// if the test succeeded, clean up
			os.Remove(TEMP_FILE)
		})
	}
}

// includeSchemaPath is the example whose messages are deliberately unreachable
// from any operation. It is kept out of openapiTests on purpose: each of the
// table-driven tests above applies one fixed option set, none of which can carry
// include_schema, and flipping GENERATE_FIXTURES would emit five fixtures for it
// that no test ever reads.
const includeSchemaPath = "examples/tests/includeschema/"

func TestOpenAPIIncludeSchema(t *testing.T) {
	tests := []struct {
		name    string
		opts    []string
		fixture string
	}{{
		// The feature is opt-in: without it, only what the operation reaches.
		name:    "Not requested",
		opts:    []string{"naming=proto"},
		fixture: "openapi.yaml",
	}, {
		// StandaloneChild is not requested; it arrives because Standalone
		// references it.
		name:    "Transitive",
		opts:    []string{"naming=proto", "include_schema=tests.includeschema.message.v1.Standalone"},
		fixture: "openapi_include_schema.yaml",
	}, {
		// The proto name of a nested message is Outer.Inner, not Outer_Inner.
		name:    "Nested",
		opts:    []string{"naming=proto", "include_schema=tests.includeschema.message.v1.Outer.Inner"},
		fixture: "openapi_include_nested.yaml",
	}, {
		// The seeded name is the formatted one, so refs still resolve.
		name:    "Fully qualified schema naming",
		opts:    []string{"naming=proto", "fq_schema_naming=1", "include_schema=tests.includeschema.message.v1.Standalone"},
		fixture: "openapi_include_fq_schema_naming.yaml",
	}, {
		// Two roots at once, and a leading dot is tolerated as in a proto type
		// reference.
		name: "Multiple",
		opts: []string{
			"naming=proto",
			"include_schema=tests.includeschema.message.v1.Standalone",
			"include_schema=.tests.includeschema.message.v1.Outer.Inner",
		},
		fixture: "openapi_include_multiple.yaml",
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := path.Join(includeSchemaPath, tt.fixture)
			args := []string{
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(includeSchemaPath, "message.proto"),
				"--openapi_out=.",
			}
			for _, opt := range tt.opts {
				args = append(args, "--openapi_opt="+opt)
			}
			if err := exec.Command("protoc", args...).Run(); err != nil {
				t.Fatalf("protoc %v failed: %+v", strings.Join(args, " "), err)
			}
			if GENERATE_FIXTURES {
				if err := CopyFixture(TEMP_FILE, fixture); err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else if err := exec.Command("diff", TEMP_FILE, fixture).Run(); err != nil {
				t.Fatalf("Diff failed: %+v", err)
			}
			os.Remove(TEMP_FILE)
		})
	}
}

func TestOpenAPIIncludeSchemaErrors(t *testing.T) {
	tests := []struct {
		name string
		opts []string
		// Protos to compile, in order; message.proto alone when unset.
		protos []string
		want   string
	}{{
		name: "Unknown message",
		opts: []string{"include_schema=tests.includeschema.message.v1.NoSuchMessage"},
		want: "no message named tests.includeschema.message.v1.NoSuchMessage in the compile set",
	}, {
		// Map entries are an implementation detail of map<> fields, not a
		// nameable type.
		name: "Map entry",
		opts: []string{"include_schema=tests.includeschema.message.v1.Standalone.ChildrenEntry"},
		want: "no message named tests.includeschema.message.v1.Standalone.ChildrenEntry in the compile set",
	}, {
		// Timestamp is expanded inline everywhere, so a component schema for it
		// would contradict every use of it.
		name: "Inline well-known type",
		opts: []string{"include_schema=google.protobuf.Timestamp"},
		want: "google.protobuf.Timestamp is expanded inline",
	}, {
		// collision.proto's Standalone is reached from an operation and formats
		// to the same schema name. Emitting the requested one is impossible: the
		// walk matches by name, so the document would keep whichever message it
		// reached first under that name.
		name:   "Cross-package name collision",
		opts:   []string{"naming=proto", "include_schema=tests.includeschema.message.v1.Standalone"},
		protos: []string{"message.proto", "collision.proto"},
		want:   `both map to schema name "Standalone"`,
	}, {
		// fieldcollision.proto reaches its Standalone through a field, so the
		// name is unclaimed when the schema is seeded and the collision only
		// surfaces while the schemas are being generated.
		name:   "Name collision discovered while generating",
		opts:   []string{"naming=proto", "include_schema=tests.includeschema.message.v1.Standalone"},
		protos: []string{"fieldcollision.proto", "message.proto"},
		want:   `tests.includeschema.message.v1.Standalone and tests.includeschema.fieldcollision.v1.Standalone both map to schema name "Standalone"`,
	}, {
		// Same clash with the protos the other way round: which message the
		// generation walk happens to reach first must not decide whether the
		// request is reported as impossible.
		name:   "Name collision discovered while generating, reversed",
		opts:   []string{"naming=proto", "include_schema=tests.includeschema.message.v1.Standalone"},
		protos: []string{"message.proto", "fieldcollision.proto"},
		want:   `tests.includeschema.message.v1.Standalone and tests.includeschema.fieldcollision.v1.Standalone both map to schema name "Standalone"`,
	}, {
		// Both Standalones are requested outright, and merged output puts them in
		// one document. fieldcollision's is reached only through a field, so no
		// operation has claimed the name yet and the request set collides with
		// itself. Sorted names make which one is reported deterministic.
		name: "Two requested messages share a schema name",
		opts: []string{
			"naming=proto",
			"include_schema=tests.includeschema.message.v1.Standalone",
			"include_schema=tests.includeschema.fieldcollision.v1.Standalone",
		},
		protos: []string{"message.proto", "fieldcollision.proto"},
		want:   `both map to schema name "Standalone"`,
	}, {
		// The default error response emits google.rpc.Status under the schema
		// name "Status" before the schemas are walked at all, so a requested
		// message of that name can never get it.
		name:   "Name taken by the default error response",
		opts:   []string{"naming=proto", "include_schema=tests.includeschema.statusname.v1.Status"},
		protos: []string{"statusname.proto"},
		want:   `schema "Status" was generated from google.rpc.Status, not the requested tests.includeschema.statusname.v1.Status`,
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			protos := tt.protos
			if protos == nil {
				protos = []string{"message.proto"}
			}
			args := []string{
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				"--openapi_out=.",
			}
			for _, proto := range protos {
				args = append(args, path.Join(includeSchemaPath, proto))
			}
			for _, opt := range tt.opts {
				args = append(args, "--openapi_opt="+opt)
			}
			// The existing tests use Run(), which discards stderr; here the
			// message is the thing under test.
			out, err := exec.Command("protoc", args...).CombinedOutput()
			if err == nil {
				os.Remove(TEMP_FILE)
				t.Fatalf("protoc succeeded, want failure. Output:\n%s", out)
			}
			if !strings.Contains(string(out), tt.want) {
				t.Fatalf("protoc error does not mention %q:\n%s", tt.want, out)
			}
		})
	}
}

// TestOpenAPIIncludeSchemaCollisionDisambiguated checks the remedy the collision
// error points at: with fq_schema_naming the two Standalone messages no longer
// share a schema name, so the requested one can be emitted alongside the other.
func TestOpenAPIIncludeSchemaCollisionDisambiguated(t *testing.T) {
	args := []string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		path.Join(includeSchemaPath, "message.proto"),
		path.Join(includeSchemaPath, "collision.proto"),
		"--openapi_out=.",
		"--openapi_opt=naming=proto",
		"--openapi_opt=fq_schema_naming=1",
		"--openapi_opt=include_schema=tests.includeschema.message.v1.Standalone",
	}
	out, err := exec.Command("protoc", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("protoc %v failed: %+v\n%s", strings.Join(args, " "), err, out)
	}
	defer os.Remove(TEMP_FILE)

	document, err := os.ReadFile(TEMP_FILE)
	if err != nil {
		t.Fatalf("read generated spec: %+v", err)
	}
	for _, want := range []string{
		"tests.includeschema.message.v1.Standalone:",
		"tests.includeschema.collision.v1.Standalone:",
	} {
		if !strings.Contains(string(document), want) {
			t.Errorf("generated spec is missing %q:\n%s", want, document)
		}
	}
}

// TestOpenAPIIncludeSchemaImportOnly covers a message declared in a .proto that
// is present only as an import. Merged mode reaches the whole compile set and
// emits it; source_relative emits a spec per generated file, so there is none
// that the declaring file owns and the request has to fail rather than be
// dropped.
func TestOpenAPIIncludeSchemaImportOnly(t *testing.T) {
	const detached = "include_schema=tests.includeschema.imported.v1.Detached"
	importer := path.Join(includeSchemaPath, "importer.proto")
	imported := path.Join(includeSchemaPath, "imported.proto")

	tempDir := "tmp_import_only"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)

	run := func(t *testing.T, outDir string, extra ...string) ([]byte, error) {
		if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
			t.Fatalf("create out directory %+v", err)
		}
		args := append([]string{
			"-I", "../../",
			"-I", "../../third_party",
			"-I", "examples",
			"--openapi_out=" + outDir,
			"--openapi_opt=naming=proto",
			"--openapi_opt=" + detached,
		}, extra...)
		return exec.Command("protoc", args...).CombinedOutput()
	}

	t.Run("Merged", func(t *testing.T) {
		outDir := path.Join(tempDir, "merged")
		out, err := run(t, outDir, importer)
		if err != nil {
			t.Fatalf("protoc failed: %+v\n%s", err, out)
		}
		document, err := os.ReadFile(path.Join(outDir, TEMP_FILE))
		if err != nil {
			t.Fatalf("read generated spec: %+v", err)
		}
		if !strings.Contains(string(document), "Detached:") {
			t.Errorf("spec is missing the imported message:\n%s", document)
		}
	})

	t.Run("Source relative", func(t *testing.T) {
		out, err := run(t, path.Join(tempDir, "sr"), importer, "--openapi_opt=output_mode=source_relative")
		if err == nil {
			t.Fatalf("protoc succeeded, want failure. Output:\n%s", out)
		}
		for _, want := range []string{
			"tests/includeschema/imported.proto",
			"protoc was not asked to generate for",
		} {
			if !strings.Contains(string(out), want) {
				t.Errorf("protoc error does not mention %q:\n%s", want, out)
			}
		}
	})

	// The remedy the error points at: once the declaring file is generated for,
	// it has a spec of its own to hold the schema.
	t.Run("Source relative with the declaring file", func(t *testing.T) {
		outDir := path.Join(tempDir, "sr_both")
		out, err := run(t, outDir, importer, imported, "--openapi_opt=output_mode=source_relative")
		if err != nil {
			t.Fatalf("protoc failed: %+v\n%s", err, out)
		}
		document, err := os.ReadFile(path.Join(outDir, "tests/includeschema/imported.openapi.yaml"))
		if err != nil {
			t.Fatalf("read declaring file's spec: %+v", err)
		}
		if !strings.Contains(string(document), "Detached:") {
			t.Errorf("declaring file's spec is missing the included schema:\n%s", document)
		}
	})
}

// TestOpenAPIIncludeSchemaSourceRelative checks that a forced schema lands only
// in the spec for the file that declares it, rather than in every per-file spec.
func TestOpenAPIIncludeSchemaSourceRelative(t *testing.T) {
	tempDir := "tmp_include"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)

	args := []string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		path.Join(includeSchemaPath, "message.proto"),
		path.Join("examples/tests/bodymapping/", "message.proto"),
		"--openapi_out=" + tempDir,
		"--openapi_opt=naming=proto",
		"--openapi_opt=output_mode=source_relative",
		"--openapi_opt=include_schema=tests.includeschema.message.v1.Standalone",
	}
	if err := exec.Command("protoc", args...).Run(); err != nil {
		t.Fatalf("protoc %v failed: %+v", strings.Join(args, " "), err)
	}

	// The output path mirrors the proto's canonical name, which "-I examples"
	// strips down to "tests/<dir>/message.proto".
	declaring, err := os.ReadFile(path.Join(tempDir, "tests/includeschema/message.openapi.yaml"))
	if err != nil {
		t.Fatalf("read declaring spec: %+v", err)
	}
	if !strings.Contains(string(declaring), "Standalone:") {
		t.Errorf("declaring file's spec is missing the included schema:\n%s", declaring)
	}

	other, err := os.ReadFile(path.Join(tempDir, "tests/bodymapping/message.openapi.yaml"))
	if err != nil {
		t.Fatalf("read other spec: %+v", err)
	}
	if strings.Contains(string(other), "Standalone:") {
		t.Errorf("included schema leaked into an unrelated file's spec:\n%s", other)
	}
}

// TestOpenAPIIncludeSchemaSourceRelativeCollision checks that two requested
// messages sharing a schema name are not a collision when they are declared in
// different files: source_relative gives each its own spec, so each definition
// gets the name to itself. An unrelated third target, which seeds nothing, must
// come through untouched rather than be failed along with them.
func TestOpenAPIIncludeSchemaSourceRelativeCollision(t *testing.T) {
	tempDir := "tmp_include_collision"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)

	args := []string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		path.Join(includeSchemaPath, "message.proto"),
		path.Join(includeSchemaPath, "fieldcollision.proto"),
		path.Join("examples/tests/bodymapping/", "message.proto"),
		"--openapi_out=" + tempDir,
		"--openapi_opt=naming=proto",
		"--openapi_opt=output_mode=source_relative",
		"--openapi_opt=include_schema=tests.includeschema.message.v1.Standalone",
		"--openapi_opt=include_schema=tests.includeschema.fieldcollision.v1.Standalone",
	}
	out, err := exec.Command("protoc", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("protoc %v failed: %+v\n%s", strings.Join(args, " "), err, out)
	}

	// Each spec has to hold the definition of its own file's Standalone, not
	// merely a schema under that name: "child" and "other" are the fields that
	// tell the two apart.
	for _, tt := range []struct {
		spec  string
		field string
	}{
		{spec: "tests/includeschema/message.openapi.yaml", field: "child:"},
		{spec: "tests/includeschema/fieldcollision.openapi.yaml", field: "other:"},
	} {
		document, err := os.ReadFile(path.Join(tempDir, tt.spec))
		if err != nil {
			t.Fatalf("read %s: %+v", tt.spec, err)
		}
		if !strings.Contains(string(document), "Standalone:") {
			t.Errorf("%s is missing the included schema:\n%s", tt.spec, document)
		}
		if !strings.Contains(string(document), tt.field) {
			t.Errorf("%s holds the wrong Standalone, expected a %q field:\n%s", tt.spec, tt.field, document)
		}
	}

	other, err := os.ReadFile(path.Join(tempDir, "tests/bodymapping/message.openapi.yaml"))
	if err != nil {
		t.Fatalf("read unrelated spec: %+v", err)
	}
	if strings.Contains(string(other), "Standalone:") {
		t.Errorf("included schema leaked into an unrelated file's spec:\n%s", other)
	}
}

// TestOpenAPIIncludeSchemaWellKnownComponent covers the well-known types that
// are emitted as components rather than inlined. The default error response
// already puts google.protobuf.Any and google.rpc.Status in the document before
// the schemas are walked, so requesting one has to be recognised as satisfied
// rather than reported missing.
func TestOpenAPIIncludeSchemaWellKnownComponent(t *testing.T) {
	tempDir := "tmp_include_wk"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)

	args := []string{
		"-I", "../../",
		"-I", "../../third_party",
		"-I", "examples",
		"examples/tests/protobuftypes/message.proto",
		"--openapi_out=" + tempDir,
		"--openapi_opt=include_schema=google.protobuf.Any",
	}
	out, err := exec.Command("protoc", args...).CombinedOutput()
	if err != nil {
		t.Fatalf("protoc %v failed: %+v\n%s", strings.Join(args, " "), err, out)
	}
	document, err := os.ReadFile(path.Join(tempDir, TEMP_FILE))
	if err != nil {
		t.Fatalf("read generated spec: %+v", err)
	}
	if !strings.Contains(string(document), "GoogleProtobufAny:") {
		t.Errorf("generated spec is missing GoogleProtobufAny:\n%s", document)
	}
}

// TestOpenAPIShadowedSchemaName covers two messages that format to the same
// schema name where only one of them is referenced. The document has to keep the
// referenced one, whichever file the walk reaches first — under source_relative
// the unreferenced message is the one declared in the file being generated for.
func TestOpenAPIShadowedSchemaName(t *testing.T) {
	tempDir := "tmp_shadowing"
	if err := os.MkdirAll(tempDir, os.ModePerm); err != nil {
		t.Fatalf("create tmp directory %+v", err)
	}
	defer os.RemoveAll(tempDir)

	for _, tt := range []struct {
		name string
		opts []string
		spec string
	}{{
		name: "Merged",
		spec: TEMP_FILE,
	}, {
		name: "Source relative",
		opts: []string{"output_mode=source_relative"},
		spec: "tests/includeschema/shadowing.openapi.yaml",
	}} {
		t.Run(tt.name, func(t *testing.T) {
			outDir := path.Join(tempDir, strings.ReplaceAll(tt.name, " ", "_"))
			if err := os.MkdirAll(outDir, os.ModePerm); err != nil {
				t.Fatalf("create out directory %+v", err)
			}
			args := []string{
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(includeSchemaPath, "shadowing.proto"),
				"--openapi_out=" + outDir,
				"--openapi_opt=naming=proto",
			}
			for _, opt := range tt.opts {
				args = append(args, "--openapi_opt="+opt)
			}
			out, err := exec.Command("protoc", args...).CombinedOutput()
			if err != nil {
				t.Fatalf("protoc %v failed: %+v\n%s", strings.Join(args, " "), err, out)
			}
			document, err := os.ReadFile(path.Join(outDir, tt.spec))
			if err != nil {
				t.Fatalf("read generated spec: %+v", err)
			}
			// "id" is the imported Detached's field, "shadow" the local one's.
			if !strings.Contains(string(document), "id:") {
				t.Errorf("spec does not hold the referenced Detached:\n%s", document)
			}
			if strings.Contains(string(document), "shadow:") {
				t.Errorf("spec holds the unreferenced Detached:\n%s", document)
			}
		})
	}
}

func TestOpenAPIDefaultResponse(t *testing.T) {
	for _, tt := range openapiTests {
		fixture := path.Join(tt.path, "openapi_default_response.yaml")
		if _, err := os.Stat(fixture); errors.Is(err, os.ErrNotExist) {
			if !GENERATE_FIXTURES {
				continue
			}
		}
		t.Run(tt.name, func(t *testing.T) {
			// Run protoc and the protoc-gen-openapi plugin to generate an OpenAPI spec with string Enums.
			err := exec.Command("protoc",
				"-I", "../../",
				"-I", "../../third_party",
				"-I", "examples",
				path.Join(tt.path, tt.protofile),
				"--openapi_out=default_response=true:.").Run()
			if err != nil {
				t.Fatalf("protoc failed: %+v", err)
			}
			if GENERATE_FIXTURES {
				err := CopyFixture(TEMP_FILE, fixture)
				if err != nil {
					t.Fatalf("Can't generate fixture: %+v", err)
				}
			} else {
				// Verify that the generated spec matches our expected version.
				err = exec.Command("diff", TEMP_FILE, fixture).Run()
				if err != nil {
					t.Fatalf("diff failed: %+v", err)
				}
			}
			// if the test succeeded, clean up
			os.Remove(TEMP_FILE)
		})
	}
}
