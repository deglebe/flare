package flare

import "encoding/json"

type (
	jsonMarshalFunc   func(any) ([]byte, error)
	jsonUnmarshalFunc func([]byte, any) error
)

var (
	jsonMarshal   jsonMarshalFunc   = json.Marshal
	jsonUnmarshal jsonUnmarshalFunc = json.Unmarshal
)

// swaps json encode/decode
func SetJSONCodec(marshal jsonMarshalFunc, unmarshal jsonUnmarshalFunc) {
	if marshal != nil {
		jsonMarshal = marshal
	}
	if unmarshal != nil {
		jsonUnmarshal = unmarshal
	}
}
