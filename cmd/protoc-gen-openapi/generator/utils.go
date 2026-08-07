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

package generator

import (
	"strings"

	"google.golang.org/protobuf/reflect/protoreflect"
)

// oauth2SchemeName is the securitySchemes key that per-operation oauth2
// requirements reference. The scheme itself is declared via the document
// annotation on the proto (components.securitySchemes).
const oauth2SchemeName = "oauth2Auth"

// authExtensionNumber is the field number of the (auth) extension on
// google.protobuf.MethodOptions. authScopesFieldNumber is the field number of
// the oauth2_scopes string within the Auth message. Referencing by number
// avoids a Go dependency on the definition module (which would be a cycle).
const (
	authExtensionNumber   protoreflect.FieldNumber = 4290001
	authScopesFieldNumber protoreflect.FieldNumber = 1
)

// oauth2Scopes reads the comma-separated (auth).oauth2_scopes off a method's
// options and returns the trimmed, non-empty scopes. The extension is resolved
// by protogen from the input file set, so it is surfaced as a recognized field.
func oauth2Scopes(opts protoreflect.ProtoMessage) []string {
	if opts == nil {
		return nil
	}
	var scopes []string
	opts.ProtoReflect().Range(func(fd protoreflect.FieldDescriptor, v protoreflect.Value) bool {
		if !fd.IsExtension() || fd.Number() != authExtensionNumber {
			return true
		}
		scopesField := v.Message().Descriptor().Fields().ByNumber(authScopesFieldNumber)
		if scopesField == nil {
			return false
		}
		for _, s := range strings.Split(v.Message().Get(scopesField).String(), ",") {
			if s = strings.TrimSpace(s); s != "" {
				scopes = append(scopes, s)
			}
		}
		return false
	})
	return scopes
}

// contains returns true if an array contains a specified string.
func contains(s []string, e string) bool {
	for _, a := range s {
		if a == e {
			return true
		}
	}
	return false
}

// appendUnique appends a string, to a string slice, if the string is not already in the slice
func appendUnique(s []string, e string) []string {
	if !contains(s, e) {
		return append(s, e)
	}
	return s
}

// singular produces the singular form of a collection name.
func singular(plural string) string {
	if strings.HasSuffix(plural, "ves") {
		return strings.TrimSuffix(plural, "ves") + "f"
	}
	if strings.HasSuffix(plural, "ies") {
		return strings.TrimSuffix(plural, "ies") + "y"
	}
	if strings.HasSuffix(plural, "s") {
		return strings.TrimSuffix(plural, "s")
	}
	return plural
}

func getValueKind(message protoreflect.MessageDescriptor) string {
	valueField := getValueField(message)
	return valueField.Kind().String()
}

func getValueField(message protoreflect.MessageDescriptor) protoreflect.FieldDescriptor {
	fields := message.Fields()
	return fields.ByName("value")
}
