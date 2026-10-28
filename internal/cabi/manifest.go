// Package cabi defines the outgoing C ABI manifest contract shared by
// generation and compatibility checking. It is independent of C FFI imports.
package cabi

const SchemaVersion = 2

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
}

type Status struct {
	OK              int32 `json:"ok"`
	Panic           int32 `json:"panic"`
	InvalidArgument int32 `json:"invalidArgument"`
}

type EnumMember struct {
	Name  string `json:"name"`
	Value string `json:"value"`
}

// Type distinguishes semantics that share a C transport. Enum names and source
// paths are intentionally absent: only the public member/value contract matters.
type Type struct {
	Kind    string       `json:"kind"`
	Members []EnumMember `json:"members,omitempty"`
}

type Function struct {
	Symbol          string   `json:"symbol"`
	Parameters      []string `json:"parameters"`
	Result          string   `json:"result"`
	ResultTransport string   `json:"resultTransport"`
	ParameterTypes  []Type   `json:"parameterTypes"`
	ResultType      Type     `json:"resultType"`
}

type FingerprintInput struct {
	SchemaVersion  int        `json:"schemaVersion"`
	GatewayVersion Version    `json:"gatewayVersion"`
	Status         Status     `json:"status"`
	Functions      []Function `json:"functions"`
}

type Manifest struct {
	SchemaVersion  int        `json:"schemaVersion"`
	GatewayVersion Version    `json:"gatewayVersion"`
	Fingerprint    string     `json:"fingerprint"`
	Status         Status     `json:"status"`
	Functions      []Function `json:"functions"`
}
