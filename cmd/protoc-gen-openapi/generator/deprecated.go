package generator

import (
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/descriptorpb"
)

func isServiceDeprecated(desc protoreflect.ServiceDescriptor) bool {
	opts := desc.Options().(*descriptorpb.ServiceOptions)
	if opts != nil && opts.Deprecated != nil {
		return *opts.Deprecated
	}
	return false
}

func isMessageDeprecated(desc protoreflect.MessageDescriptor) bool {
	opts := desc.Options().(*descriptorpb.MessageOptions)
	if opts != nil && opts.Deprecated != nil {
		return *opts.Deprecated
	}
	return false
}

func isOperationDeprecated(desc protoreflect.MethodDescriptor) bool {
	opts := desc.Options().(*descriptorpb.FieldOptions)
	if opts != nil && opts.Deprecated != nil {
		return *opts.Deprecated
	}
	return false
}

func isFieldDeprecated(desc protoreflect.FieldDescriptor) bool {
	opts := desc.Options().(*descriptorpb.FieldOptions)
	if opts != nil && opts.Deprecated != nil {
		return *opts.Deprecated
	}
	return false
}
