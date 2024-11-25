package grpcx

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// RawBytesCodec sets the received bytes as-is to the target,
// whether it is a byte slice or a proto.Message.
// For proto.Message, it uses proto.Marshal and proto.Unmarshal.
type RawBytesCodec struct{}

// Marshal returns the received byte slice as is.
func (RawBytesCodec) Marshal(v any) ([]byte, error) {
	if v == nil {
		return nil, nil
	}

	switch v := v.(type) {
	case []byte:
		return v, nil
	case *[]byte:
		return *v, nil
	case proto.Message:
		return proto.Marshal(v)
	default:
		return nil, fmt.Errorf("failed to marshal: %v is of type %T, not *[]byte, nor proto.Message", v, v)
	}
}

// Unmarshal sets the received bytes as is to the target.
func (RawBytesCodec) Unmarshal(data []byte, v any) error {
	if data == nil || v == nil {
		return nil
	}

	switch v := v.(type) {
	case *[]byte:
		*v = data
		return nil
	case proto.Message:
		return proto.Unmarshal(data, v)
	default:
		return fmt.Errorf("failed to unmarshal: %v is of type %T, not *[]byte, nor proto.Message", v, v)
	}
}

// Name returns the name of the codec.
func (RawBytesCodec) Name() string { return "groxy-raw-bytes" }
