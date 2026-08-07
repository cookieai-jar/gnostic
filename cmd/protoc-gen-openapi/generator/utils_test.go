package generator

import (
	"slices"
	"testing"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

// authExtType builds the (auth) extension the way protogen surfaces it at
// runtime: a dynamicpb extension type on google.protobuf.MethodOptions with
// field number 4290001 and a nested Auth message carrying oauth2_scopes.
func authExtType(t *testing.T) (protoreflect.ExtensionType, protoreflect.MessageDescriptor) {
	t.Helper()
	fdp := &descriptorpb.FileDescriptorProto{
		Name:       proto.String("auth_test.proto"),
		Syntax:     proto.String("proto3"),
		Dependency: []string{"google/protobuf/descriptor.proto"},
		MessageType: []*descriptorpb.DescriptorProto{{
			Name: proto.String("Auth"),
			Field: []*descriptorpb.FieldDescriptorProto{{
				Name:     proto.String("oauth2_scopes"),
				Number:   proto.Int32(int32(authScopesFieldNumber)),
				Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
				Type:     descriptorpb.FieldDescriptorProto_TYPE_STRING.Enum(),
				JsonName: proto.String("oauth2Scopes"),
			}},
		}},
		Extension: []*descriptorpb.FieldDescriptorProto{{
			Name:     proto.String("auth"),
			Number:   proto.Int32(int32(authExtensionNumber)),
			Label:    descriptorpb.FieldDescriptorProto_LABEL_OPTIONAL.Enum(),
			Type:     descriptorpb.FieldDescriptorProto_TYPE_MESSAGE.Enum(),
			TypeName: proto.String(".Auth"),
			Extendee: proto.String(".google.protobuf.MethodOptions"),
		}},
	}
	fd, err := protodesc.NewFile(fdp, protoregistry.GlobalFiles)
	if err != nil {
		t.Fatalf("build file descriptor: %v", err)
	}
	return dynamicpb.NewExtensionType(fd.Extensions().Get(0)), fd.Messages().Get(0)
}

func optionsWithScopes(t *testing.T, raw string) *descriptorpb.MethodOptions {
	t.Helper()
	extType, authDesc := authExtType(t)
	authMsg := dynamicpb.NewMessage(authDesc)
	authMsg.Set(authDesc.Fields().ByNumber(authScopesFieldNumber), protoreflect.ValueOfString(raw))
	opts := &descriptorpb.MethodOptions{}
	proto.SetExtension(opts, extType, authMsg)
	return opts
}

func TestOauth2Scopes(t *testing.T) {
	tests := []struct {
		name string
		opts *descriptorpb.MethodOptions
		want []string
	}{
		{name: "nil options", opts: nil, want: nil},
		{name: "no auth option", opts: &descriptorpb.MethodOptions{}, want: nil},
		{name: "empty scopes", opts: optionsWithScopes(t, ""), want: nil},
		{name: "single scope", opts: optionsWithScopes(t, "assessments:risks:read"), want: []string{"assessments:risks:read"}},
		{
			name: "multiple scopes with whitespace",
			opts: optionsWithScopes(t, "assessments:risks:read, assessments:queries:read ,team:read"),
			want: []string{"assessments:risks:read", "assessments:queries:read", "team:read"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got []string
			if tt.opts != nil {
				got = oauth2Scopes(tt.opts)
			} else {
				got = oauth2Scopes(nil)
			}
			if !slices.Equal(got, tt.want) {
				t.Fatalf("oauth2Scopes() = %v, want %v", got, tt.want)
			}
		})
	}
}
