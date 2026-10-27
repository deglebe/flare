package flare

import "encoding/json"

type (
	jsonMarshalFunc   func(any) ([]byte, error)
	jsonUnmarshalFunc func([]byte, any) error
)

// defaults to encoding/json; overridden by SetJSONCodec / flare_sonic
var (
	jsonMarshal   jsonMarshalFunc   = json.Marshal
	jsonUnmarshal jsonUnmarshalFunc = json.Unmarshal
)

// swaps json encode/decode (nil arg keeps current)
func SetJSONCodec(marshal jsonMarshalFunc, unmarshal jsonUnmarshalFunc) {
	if marshal != nil {
		jsonMarshal = marshal
	}
	if unmarshal != nil {
		jsonUnmarshal = unmarshal
	}
}
