//go:build flare_sonic

package flare

import "github.com/bytedance/sonic"

func init() {
	SetJSONCodec(sonic.Marshal, sonic.Unmarshal)
}
