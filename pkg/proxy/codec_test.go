package proxy

import (
	"testing"

	"github.com/stretchr/testify/require"
	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/protobuf/proto"
)

func TestRawBytesCodec_Marshal(t *testing.T) {
	t.Run("proto message", func(t *testing.T) {
		bts, err := RawBytesCodec{}.Marshal(&errdetails.RequestInfo{RequestId: "1"})
		require.NoError(t, err)
		expected, err := proto.Marshal(&errdetails.RequestInfo{RequestId: "1"})
		require.NoError(t, err)
		require.Equal(t, expected, bts)
	})

	t.Run("byte slice", func(t *testing.T) {
		bts, err := RawBytesCodec{}.Marshal(&[]byte{1, 2, 3})
		require.NoError(t, err)
		require.Equal(t, []byte{1, 2, 3}, bts)
	})
}

func TestRawBytesCodec_Unmarshal(t *testing.T) {
	t.Run("proto message", func(t *testing.T) {
		bts, err := proto.Marshal(&errdetails.RequestInfo{RequestId: "1"})
		require.NoError(t, err)

		msg := &errdetails.RequestInfo{}
		require.NoError(t, RawBytesCodec{}.Unmarshal(bts, msg))
		require.Truef(t, proto.Equal(&errdetails.RequestInfo{RequestId: "1"}, msg), "got: %v", msg)
	})

	t.Run("byte slice", func(t *testing.T) {
		var bts []byte
		err := RawBytesCodec{}.Unmarshal([]byte{1, 2, 3}, &bts)
		require.NoError(t, err)
		require.Equal(t, []byte{1, 2, 3}, bts)
	})
}
