package generator

import (
	"fmt"
	"sort"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// seededSchema pairs a formatted schema name with the message it was requested
// for. The generator matches messages to required schemas by name alone, so the
// proto name has to be carried along for checkIncludedSchemasGenerated to tell
// whether the right definition was emitted.
type seededSchema struct {
	schemaName string
	fullName   string
}

// seedIncludedSchemas marks every message named by the include_schema option as
// required, so the schema generation loop emits it even though no operation
// references it. It returns what it seeded, for checkIncludedSchemasGenerated
// to verify afterwards.
func (g *OpenAPIv3Generator) seedIncludedSchemas() ([]seededSchema, error) {
	requested := normalizeIncludeSchemas(g.conf.IncludeSchemas)
	if len(requested) == 0 {
		return nil, nil
	}

	byName := messagesByFullName(g.plugin.Files)
	generated := generatedFilePaths(g.inputFiles)

	// Resolve everything before seeding anything, so a bad list fails as a whole
	// rather than half-applying.
	descriptors := make([]protoreflect.MessageDescriptor, 0, len(requested))
	var unknown, inlined, importOnly []string
	for _, name := range requested {
		// Checked before the lookup: these are inline-only whether or not the
		// compile set happens to contain them, and "cannot be a component" is
		// the more useful message than "not found".
		if isInlineWellKnownType(name) {
			inlined = append(inlined, name)
			continue
		}
		message, ok := byName[name]
		if !ok {
			unknown = append(unknown, name)
			continue
		}
		// A file protoc was never asked to generate for gets no document of its
		// own, so the filter below would skip this message on every invocation
		// and it would land nowhere. Only source_relative reaches this: in
		// merged mode g.inputFiles is the whole compile set, so every path is
		// already in generated.
		if path := message.ParentFile().Path(); !generated[path] && !isGenerationTarget(g.plugin.Files, path) {
			importOnly = append(importOnly, fmt.Sprintf("%s (%s)", name, path))
			continue
		}
		descriptors = append(descriptors, message)
	}
	if len(inlined) > 0 {
		return nil, errors.Errorf(
			"include_schema: %s %s expanded inline at each reference site and cannot be emitted as a component schema",
			strings.Join(inlined, ", "), plural(len(inlined), "is", "are"))
	}
	if len(unknown) > 0 {
		return nil, errors.Errorf(
			"include_schema: no message named %s in the compile set; the declaring .proto must be passed to protoc/buf or be reachable from one that is",
			strings.Join(unknown, ", "))
	}
	if len(importOnly) > 0 {
		return nil, errors.Errorf(
			"include_schema: %s %s declared in a .proto that protoc was not asked to generate for; output_mode=source_relative emits a spec per generated file, so there is none to add %s to — pass the declaring .proto to protoc/buf as well, or use the default merged output mode",
			strings.Join(importOnly, ", "), plural(len(importOnly), "is", "are"), plural(len(importOnly), "it", "them"))
	}

	// Two messages in different packages can format to the same schema name when
	// fq_schema_naming is off. Only the first would ever be emitted, so the rest
	// would silently resolve to the wrong definition. The clash is per document:
	// source_relative gives each declaring file a spec of its own, so messages
	// that land in different specs never contend for the name.
	owner := make(map[string]string, len(descriptors))
	var seeded []seededSchema
	for _, message := range descriptors {
		// In source_relative mode one generator runs per input file while
		// g.plugin.Files stays the full compile set. Seeding unconditionally
		// would copy the schema into every per-file spec, so only the spec for
		// the file that declares the message gets it. Everything below concerns
		// the document being built, so it comes after this filter.
		if !generated[message.ParentFile().Path()] {
			continue
		}

		schemaName := g.reflect.formatMessageName(message)
		fullName := string(message.FullName())
		if other, ok := owner[schemaName]; ok {
			return nil, collisionError(other, fullName, schemaName)
		}
		owner[schemaName] = fullName

		// The operations have already been walked, so anything they reference is
		// recorded by now. A name another message has already claimed cannot be
		// given away: every reference to it means that message's definition.
		if other, ok := g.reflect.requiredSchemaOwners[schemaName]; ok && other != fullName {
			return nil, collisionError(other, fullName, schemaName)
		}

		g.reflect.schemaReferenceForMessage(message)
		seeded = append(seeded, seededSchema{schemaName: schemaName, fullName: fullName})
	}
	return seeded, nil
}

func collisionError(first, second, schemaName string) error {
	return errors.Errorf("include_schema: %s", collisionMessage(first, second, schemaName))
}

func collisionMessage(first, second, schemaName string) string {
	return fmt.Sprintf(
		"%s and %s both map to schema name %q; use fq_schema_naming=true to disambiguate",
		first, second, schemaName)
}

// checkIncludedSchemasGenerated verifies each seeded schema made it into the
// document, that it was built from the message that was asked for, and that no
// other message wanted the same name. Both go undetected in seedIncludedSchemas:
// the default error response emits google.rpc.Status and google.protobuf.Any
// under their own names before any seeding happens, and a message reached only
// through a field claims its name only once the schemas are being generated.
func (g *OpenAPIv3Generator) checkIncludedSchemasGenerated(seeded []seededSchema) error {
	var problems []string
	for _, s := range seeded {
		owner, ok := g.generatedSchemaOwners[s.schemaName]
		switch {
		case !ok:
			problems = append(problems, fmt.Sprintf("no schema was generated for %s", s.schemaName))
		case owner != s.fullName:
			problems = append(problems, fmt.Sprintf(
				"schema %q was generated from %s, not the requested %s; use fq_schema_naming=true to disambiguate",
				s.schemaName, owner, s.fullName))
		default:
			if others := g.reflect.schemaNameConflicts[s.schemaName]; len(others) > 0 {
				sort.Strings(others)
				problems = append(problems, collisionMessage(s.fullName, others[0], s.schemaName))
			}
		}
	}
	if len(problems) > 0 {
		return errors.Errorf("include_schema: %s", strings.Join(problems, "; "))
	}
	return nil
}

// normalizeIncludeSchemas trims each entry, drops empties, tolerates the
// leading dot of a proto type reference, and deduplicates. The result is sorted
// so errors list names in a stable order.
func normalizeIncludeSchemas(names StringArray) []string {
	seen := make(map[string]bool, len(names))
	out := make([]string, 0, len(names))
	for _, name := range names {
		name = strings.TrimPrefix(strings.TrimSpace(name), ".")
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

// messagesByFullName indexes every message in the compile set — imports
// included — by its fully-qualified proto name. Synthetic map entry messages
// are skipped: protogen surfaces a Foo.BarEntry for every map<> field, and
// those are an implementation detail rather than a nameable type.
func messagesByFullName(files []*protogen.File) map[string]protoreflect.MessageDescriptor {
	byName := make(map[string]protoreflect.MessageDescriptor)
	var walk func(messages []*protogen.Message)
	walk = func(messages []*protogen.Message) {
		for _, message := range messages {
			if message.Desc.IsMapEntry() {
				continue
			}
			byName[string(message.Desc.FullName())] = message.Desc
			walk(message.Messages)
		}
	}
	for _, file := range files {
		walk(file.Messages)
	}
	return byName
}

// generatedFilePaths is the set of proto paths this generator invocation is
// producing a document for.
func generatedFilePaths(files []*protogen.File) map[string]bool {
	paths := make(map[string]bool, len(files))
	for _, file := range files {
		paths[file.Desc.Path()] = true
	}
	return paths
}

// isGenerationTarget reports whether protoc was asked to generate for path, as
// opposed to merely compiling it as an import.
func isGenerationTarget(files []*protogen.File, path string) bool {
	for _, file := range files {
		if file.Desc.Path() == path {
			return file.Generate
		}
	}
	return false
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
