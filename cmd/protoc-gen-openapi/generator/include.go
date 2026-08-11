package generator

import (
	"sort"
	"strings"

	"github.com/pkg/errors"
	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// seedIncludedSchemas marks every message named by the include_schema option as
// required, so the schema generation loop emits it even though no operation
// references it. It returns the formatted schema names it seeded, for
// checkIncludedSchemasGenerated to verify afterwards.
func (g *OpenAPIv3Generator) seedIncludedSchemas() ([]string, error) {
	requested := normalizeIncludeSchemas(g.conf.IncludeSchemas)
	if len(requested) == 0 {
		return nil, nil
	}

	byName := messagesByFullName(g.plugin.Files)
	generated := generatedFilePaths(g.inputFiles)

	// Resolve everything before seeding anything, so a bad list fails as a whole
	// rather than half-applying.
	descriptors := make([]protoreflect.MessageDescriptor, 0, len(requested))
	var unknown, inlined []string
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

	// Two messages in different packages can format to the same schema name when
	// fq_schema_naming is off. Only the first would ever be emitted, so the rest
	// would silently resolve to the wrong definition.
	owner := make(map[string]string, len(descriptors))
	var seeded []string
	for _, message := range descriptors {
		schemaName := g.reflect.formatMessageName(message)
		fullName := string(message.FullName())
		if other, ok := owner[schemaName]; ok {
			return nil, errors.Errorf(
				"include_schema: %s and %s both map to schema name %q; use fq_schema_naming=true to disambiguate",
				other, fullName, schemaName)
		}
		owner[schemaName] = fullName

		// In source_relative mode one generator runs per input file while
		// g.plugin.Files stays the full compile set. Seeding unconditionally
		// would copy the schema into every per-file spec, so only the spec for
		// the file that declares the message gets it.
		if !generated[message.ParentFile().Path()] {
			continue
		}

		g.reflect.schemaReferenceForMessage(message)
		seeded = append(seeded, schemaName)
	}
	return seeded, nil
}

// checkIncludedSchemasGenerated verifies the seeded schemas actually made it
// into the document. A seeded name that produced nothing means the walk matched
// a different message onto the same formatted name, which would leave the
// document holding the wrong definition under the requested name.
func (g *OpenAPIv3Generator) checkIncludedSchemasGenerated(seeded []string) error {
	var missing []string
	for _, name := range seeded {
		if !contains(g.generatedSchemas, name) {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return errors.Errorf("include_schema: no schema was generated for %s", strings.Join(missing, ", "))
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

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}
