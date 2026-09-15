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
//

package main

import (
	"flag"
	"path/filepath"
	"strings"

	"github.com/google/gnostic/cmd/protoc-gen-openapi/generator"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/pluginpb"
)

var flags flag.FlagSet

var visibilityList generator.VisibilityArray

var includeSchemaList generator.StringArray

func main() {
	flags.Var(&visibilityList, "visibility", `visibility. options: 'public','preview','private'`)
	flags.Var(&includeSchemaList, "include_schema", `fully-qualified proto message name to emit as a component schema even if no operation references it. Repeatable.`)
	conf := generator.Configuration{
		Version:         flags.String("version", "0.0.1", "version number text, e.g. 1.2.3"),
		Title:           flags.String("title", "", "name of the API"),
		Description:     flags.String("description", "", "description of the API"),
		Naming:          flags.String("naming", "json", `naming convention. Use "proto" for passing names directly from the proto files`),
		FQSchemaNaming:  flags.Bool("fq_schema_naming", false, `schema naming convention. If "true", generates fully-qualified schema names by prefixing them with the proto message package name`),
		EnumType:        flags.String("enum_type", "integer", `type for enum serialization. Use "string" for string-based serialization`),
		CircularDepth:   flags.Int("depth", 2, "depth of recursion for circular messages"),
		DefaultResponse: flags.Bool("default_response", true, `add default response. If "true", automatically adds a default response to operations which use the google.rpc.Status message. Useful if you use envoy or grpc-gateway to transcode as they use this type for their default error responses.`),
		OutputMode:      flags.String("output_mode", "merged", `output generation mode. By default, a single openapi.yaml is generated at the out folder. Use "source_relative' to generate a separate '[inputfile].openapi.yaml' next to each '[inputfile].proto'.`),
		Filename:        flags.String("filename", "openapi.yaml", `name of the output file. Default is "openapi.yaml"`),

		OAuth2ScopesExtension:  flags.String("oauth2_scopes_extension", "", `method-option extension holding comma-separated OAuth2 scopes, as "<extension-full-name>.<field-name>" (e.g. "auth.oauth2_scopes"). When set, scopes are emitted as operation tags and OAuth2 security requirements.`),
		OAuth2SchemeName:       flags.String("oauth2_scheme_name", "oauth2", `name of the synthesized OAuth2 security scheme`),
		OAuth2AuthorizationURL: flags.String("oauth2_authorization_url", "", `authorizationUrl for the synthesized OAuth2 scheme. When set, an authorizationCode flow is used; otherwise a clientCredentials flow.`),
		OAuth2TokenURL:         flags.String("oauth2_token_url", "", `tokenUrl for the synthesized OAuth2 scheme`),
	}
	opts := protogen.Options{
		ParamFunc: flags.Set,
	}

	opts.Run(func(plugin *protogen.Plugin) error {
		// slices need to be set here: flags.Var does not return the parsed slice
		conf.Visibility = visibilityList
		conf.IncludeSchemas = includeSchemaList

		// Enable "optional" keyword in front of type (e.g. optional string label = 1;)
		plugin.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)
		if *conf.OutputMode == "source_relative" {
			for _, file := range plugin.Files {
				if !file.Generate {
					continue
				}
				outfileName := strings.TrimSuffix(file.Desc.Path(), filepath.Ext(file.Desc.Path())) + "." + *conf.Filename
				outputFile := plugin.NewGeneratedFile(outfileName, "")
				gen := generator.NewOpenAPIv3Generator(plugin, conf, []*protogen.File{file})
				if err := gen.Run(outputFile); err != nil {
					return err
				}
			}
		} else {
			outputFile := plugin.NewGeneratedFile(*conf.Filename, "")
			return generator.NewOpenAPIv3Generator(plugin, conf, plugin.Files).Run(outputFile)
		}
		return nil
	})
}
