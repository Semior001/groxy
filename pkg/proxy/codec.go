package proxy

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// RawBytesCodec sets the received bytes as is to the target,
// which must always be a pointer to a byte slice.
type RawBytesCodec struct{}

// Marshal returns the received byte slice as is.
func (RawBytesCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	if msg, ok := v.(proto.Message); ok {
		return proto.Marshal(msg)
	}

	if bts, ok := v.(*[]byte); ok {
		return *bts, nil
	}

	return nil, fmt.Errorf("failed to marshal: %v is not type of *[]byte, nor proto.Message", v)
}

// Unmarshal sets the received bytes as is to the target.
func (RawBytesCodec) Unmarshal(data []byte, v any) error {
	if data == nil {
		return nil
	}

	if bts, ok := v.(proto.Message); ok {
		return proto.Unmarshal(data, bts)
	}

	if bts, ok := v.(*[]byte); ok {
		*bts = data
		return nil
	}

	return fmt.Errorf("failed to unmarshal: %v is not type of *[]byte, nor proto.Message", v)
}

// Name returns the name of the codec.
func (RawBytesCodec) Name() string { return "groxy-raw-bytes" }
