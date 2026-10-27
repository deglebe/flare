//go:build flare_sonic

package flare

import "github.com/bytedance/sonic"

// init sets the json codec to use sonic for marshalling/unmarshalling
func init() {
	SetJSONCodec(sonic.Marshal, sonic.Unmarshal)
}
