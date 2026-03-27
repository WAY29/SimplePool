package openapi

import _ "embed"

//go:embed openapi.json
var spec []byte

func JSON() []byte {
	return spec
}
