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

package generator

import (
	"fmt"
	"sort"
	"strings"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/dynamicpb"

	v3 "github.com/google/gnostic/openapiv3"
)

// resolveScopesExtension resolves the configured OAuth2 scopes method-option
// extension from the request's descriptors. It is generic: the extension is
// located by its proto full name so no Go binding needs to be linked in.
func (g *OpenAPIv3Generator) resolveScopesExtension() error {
	if g.conf.OAuth2ScopesExtension == nil || *g.conf.OAuth2ScopesExtension == "" {
		return nil
	}

	spec := *g.conf.OAuth2ScopesExtension
	dot := strings.LastIndex(spec, ".")
	if dot <= 0 || dot == len(spec)-1 {
		return fmt.Errorf("oauth2_scopes_extension %q must be <extension-full-name>.<field-name>", spec)
	}
	extName := protoreflect.FullName(spec[:dot])
	g.scopesFieldName = protoreflect.Name(spec[dot+1:])

	extDesc := g.findExtension(extName)
	if extDesc == nil {
		return fmt.Errorf("oauth2_scopes_extension %q: extension %q not found in the compiled descriptors", spec, extName)
	}
	if extDesc.Message() == nil || extDesc.Message().Fields().ByName(g.scopesFieldName) == nil {
		return fmt.Errorf("oauth2_scopes_extension %q: extension %q has no message field %q", spec, extName, g.scopesFieldName)
	}
	g.scopesExtType = dynamicpb.NewExtensionType(extDesc)

	// The extension is not linked into this binary, so method options carry it
	// as unknown bytes. Register the dynamic type in a resolver used to re-decode
	// each method's options (see scopesForMethod).
	g.scopesTypes = new(protoregistry.Types)
	if err := g.scopesTypes.RegisterExtension(g.scopesExtType); err != nil {
		return fmt.Errorf("oauth2_scopes_extension %q: %w", spec, err)
	}
	return nil
}

// findExtension searches every file in the request (including imports) for an
// extension with the given full name.
func (g *OpenAPIv3Generator) findExtension(name protoreflect.FullName) protoreflect.ExtensionDescriptor {
	for _, file := range g.plugin.Files {
		if ext := findExtensionInContainer(file.Desc, name); ext != nil {
			return ext
		}
	}
	return nil
}

type extensionContainer interface {
	Extensions() protoreflect.ExtensionDescriptors
	Messages() protoreflect.MessageDescriptors
}

func findExtensionInContainer(c extensionContainer, name protoreflect.FullName) protoreflect.ExtensionDescriptor {
	exts := c.Extensions()
	for i := 0; i < exts.Len(); i++ {
		if ext := exts.Get(i); ext.FullName() == name {
			return ext
		}
	}
	msgs := c.Messages()
	for i := 0; i < msgs.Len(); i++ {
		if ext := findExtensionInContainer(msgs.Get(i), name); ext != nil {
			return ext
		}
	}
	return nil
}

// scopesForMethod reads the comma-separated OAuth2 scopes from the method's
// options extension. Returns nil when the feature is disabled or unset.
func (g *OpenAPIv3Generator) scopesForMethod(method *protogen.Method) []string {
	if g.scopesExtType == nil {
		return nil
	}
	opts := method.Desc.Options()
	if opts == nil {
		return nil
	}

	// The extension is not linked into this binary, so it lives as unknown bytes
	// on the options. Re-decode through a resolver that knows it, into a dynamic
	// message so the extension field becomes reflectable.
	raw, err := proto.Marshal(opts)
	if err != nil {
		return nil
	}
	decoded := dynamicpb.NewMessage(opts.ProtoReflect().Descriptor())
	if err := (proto.UnmarshalOptions{Resolver: g.scopesTypes}).Unmarshal(raw, decoded); err != nil {
		return nil
	}

	xd := g.scopesExtType.TypeDescriptor()
	if !decoded.Has(xd) {
		return nil
	}
	inner := decoded.Get(xd).Message()
	fd := inner.Descriptor().Fields().ByName(g.scopesFieldName)
	if fd == nil {
		return nil
	}

	var scopes []string
	for _, s := range strings.Split(inner.Get(fd).String(), ",") {
		if s = strings.TrimSpace(s); s != "" {
			scopes = append(scopes, s)
		}
	}
	return scopes
}

// addOAuth2ScopesToOperationV3 appends each scope as an operation tag and an
// any-of OAuth2 security requirement (one requirement per scope).
func (g *OpenAPIv3Generator) addOAuth2ScopesToOperationV3(op *v3.Operation, method *protogen.Method) {
	scheme := g.oauth2SchemeName()
	for _, scope := range g.scopesForMethod(method) {
		g.seenScopes[scope] = true
		if !contains(op.Tags, scope) {
			op.Tags = append(op.Tags, scope)
		}
		op.Security = append(op.Security, &v3.SecurityRequirement{
			AdditionalProperties: []*v3.NamedStringArray{{
				Name:  scheme,
				Value: &v3.StringArray{Value: []string{scope}},
			}},
		})
	}
}

// sortedScopes returns the discovered scopes in stable order.
func (g *OpenAPIv3Generator) sortedScopes() []string {
	scopes := make([]string, 0, len(g.seenScopes))
	for s := range g.seenScopes {
		scopes = append(scopes, s)
	}
	sort.Strings(scopes)
	return scopes
}

// addOAuth2TagsToDocumentV3 declares each discovered scope as a document tag.
func (g *OpenAPIv3Generator) addOAuth2TagsToDocumentV3(d *v3.Document) {
	for _, s := range g.sortedScopes() {
		d.Tags = append(d.Tags, &v3.Tag{Name: s})
	}
}

// addOAuth2SchemeToDocumentV3 synthesizes the single OAuth2 security scheme
// covering every discovered scope.
func (g *OpenAPIv3Generator) addOAuth2SchemeToDocumentV3(d *v3.Document) {
	if len(g.seenScopes) == 0 {
		return
	}

	scopeMap := &v3.Strings{}
	for _, s := range g.sortedScopes() {
		scopeMap.AdditionalProperties = append(scopeMap.AdditionalProperties, &v3.NamedString{Name: s, Value: s})
	}

	flow := &v3.OauthFlow{
		AuthorizationUrl: g.strConf(g.conf.OAuth2AuthorizationURL),
		TokenUrl:         g.strConf(g.conf.OAuth2TokenURL),
		Scopes:           scopeMap,
	}
	flows := &v3.OauthFlows{}
	if flow.AuthorizationUrl != "" {
		flows.AuthorizationCode = flow
	} else {
		flows.ClientCredentials = flow
	}

	scheme := &v3.NamedSecuritySchemeOrReference{
		Name: g.oauth2SchemeName(),
		Value: &v3.SecuritySchemeOrReference{
			Oneof: &v3.SecuritySchemeOrReference_SecurityScheme{
				SecurityScheme: &v3.SecurityScheme{Type: "oauth2", Flows: flows},
			},
		},
	}

	if d.Components == nil {
		d.Components = &v3.Components{}
	}
	if d.Components.SecuritySchemes == nil {
		d.Components.SecuritySchemes = &v3.SecuritySchemesOrReferences{}
	}
	d.Components.SecuritySchemes.AdditionalProperties = append(d.Components.SecuritySchemes.AdditionalProperties, scheme)
}

func (g *OpenAPIv3Generator) oauth2SchemeName() string {
	if name := g.strConf(g.conf.OAuth2SchemeName); name != "" {
		return name
	}
	return "oauth2"
}

func (g *OpenAPIv3Generator) strConf(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
