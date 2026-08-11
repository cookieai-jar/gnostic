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
		want string
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
	}}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
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
