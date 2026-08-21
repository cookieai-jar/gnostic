# protoc-gen-openapi

This directory contains a protoc plugin that generates an
OpenAPI description for a REST API that corresponds to a
Protocol Buffer service.

Installation:

    go install github.com/google/gnostic/cmd/protoc-gen-openapi@latest

Usage:

	protoc sample.proto -I=. --openapi_out=.

This runs the plugin for a file named `sample.proto` which 
refers to additional .proto files in the same directory as
`sample.proto`. Output is written to the current directory.

## options

1. `version`: version number text, e.g. 1.2.3
   - **default**: `0.0.1`
2. `title`: name of the API
   - **default**: empty string or service name if there is only one service
3. `description`: description of the API
   - **default**: empty string or service description if there is only one service
4. `naming`: naming convention. Use "proto" for passing names directly from the proto files
   - **default**: `json`
   - `json`: will turn field `updated_at` to `updatedAt`
   - `proto`: keep field `updated_at` as it is
5. `fq_schema_naming`: schema naming convention. If "true", generates fully-qualified schema names by prefixing them with the proto message package name
   - **default**: false
   - `false`: keep message `Book` as it is
   - `true`: turn message `Book` to `google.example.library.v1.Book`, it is useful when there are same named message in different package
6. `enum_type`: type for enum serialization. Use "string" for string-based serialization
   - **default**: `integer`
   - `integer`: setting type to `integer`
      ```yaml
      schema:
        type: integer
        format: enum
      ```
   - `string`: setting type to `string`, and list available values in `enum`
      ```yaml
      schema:
        enum:
          - UNKNOWN_KIND
          - KIND_1
          - KIND_2
        type: string
        format: enum
      ```
7. `depth`: depth of recursion for circular messages
   - **default**: 2, this depth only used in query parameters, usually 2 is enough
8. `default_response`: add default response. If "true", automatically adds a default response to operations which use the google.rpc.Status message.
   Useful if you use envoy or grpc-gateway to transcode as they use this type for their default error responses.
   - **default**: true, this option will add this default response for each method as following:
      ```yaml
      default:
        description: Default error response
          content:
            application/json:
              schema:
                $ref: '#/components/schemas/google.rpc.Status'
      ```
9. `visibility`: `google.api.visibility` restriction to include, e.g. `PUBLIC`. Repeatable.
   Filters services and methods, and therefore transitively the schemas they reach; it never filters a schema directly.
   - **default**: none, which shows everything
10. `output_mode`: output generation mode
    - **default**: `merged`, a single file at the out folder
    - `source_relative`: a separate `[inputfile].openapi.yaml` next to each `[inputfile].proto`
11. `filename`: name of the output file
    - **default**: `openapi.yaml`
12. `include_schema`: fully-qualified proto **message** name to emit as a component schema even
    though no operation references it. Repeatable. Messages the named one references are pulled in
    transitively, so only the roots need listing.
    - **default**: none
    - Without it, a message reachable from no HTTP operation gets no schema — discovery walks
      outward from service methods:
      ```sh
      protoc ... --openapi_opt=include_schema=example.v1.VendorConfig
      ```
    - Use the proto name, so a nested message is `pkg.Outer.Inner` (it is emitted under the schema
      name `Outer_Inner`). A leading `.` is accepted.
    - The declaring `.proto` must be in the compile set — passed to protoc/buf, or imported by
      something that is. Otherwise the plugin never sees it and the build fails naming the message.
    - Enums are out of scope: they are expanded inline at each reference site, never as components.
      The same is true of `google.protobuf.Timestamp`, `Duration`, `Struct`, `FieldMask`, `Empty`,
      the wrapper types, `google.type.Date`/`DateTime` and `google.api.HttpBody`, so requesting one
      is an error rather than a contradictory second definition.
    - Under `output_mode=source_relative` the schema is added only to the spec generated for the
      file that declares it, not to every per-file spec. That mode therefore needs the declaring
      `.proto` to be one protoc generates for: a message reachable only as an import has no spec to
      go in, and is an error rather than a silent omission. Merged output has no such restriction.
    - Schemas are keyed by name, so requesting a message whose schema name another emitted message
      also formats to is an error: only one definition can live under that name, and it would not
      reliably be the one asked for. `fq_schema_naming=true` prefixes the package and resolves it.
      The clash is judged per output document, so under `output_mode=source_relative` two such
      messages are fine as long as they are declared in different files. With `default_response`
      on, `google.rpc.Status` and `google.protobuf.Any` already hold their own schema names, so a
      message formatting to one of those clashes with them.
    - Pass the option once per message rather than comma-separating names: `protoc` splits
      `--openapi_opt` on `,`, so a value may never contain one.

`buf` users should set `strategy: all` for this plugin. With the default `strategy: directory` it is
invoked once per directory, and in the default `merged` output mode each invocation overwrites the
same `openapi.yaml`.
