// Copyright 2021 github.com/gagliardetto
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 dfuse Platform Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bin

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTypeID(t *testing.T) {
	{
		ha := Sighash(SIGHASH_GLOBAL_NAMESPACE, "hello")
		vid := TypeIDFromSighash(ha)
		require.Equal(t, ha, vid.Bytes())
		require.True(t, vid.Equal(ha))
	}
	{
		expected := uint32(66)
		vid := TypeIDFromUint32(expected, binary.LittleEndian)

		got := Uint32FromTypeID(vid, binary.LittleEndian)
		require.Equal(t, expected, got)
		require.Equal(t, expected, vid.Uint32())
	}
	{
		expected := uint32(66)
		vid := TypeIDFromUvarint32(expected)

		got := Uvarint32FromTypeID(vid)
		require.Equal(t, expected, got)
		require.Equal(t, expected, vid.Uvarint32())
	}
	{
		{
			vid := TypeIDFromBytes([]byte{})
			expected := []byte{0, 0, 0, 0, 0, 0, 0, 0}
			require.Equal(t, expected, vid.Bytes())
		}
		{
			expected := []byte{1, 2, 3, 4, 5, 6, 7, 8}
			vid := TypeIDFromBytes(expected)
			require.Equal(t, expected, vid.Bytes())
		}
	}
	{
		expected := uint8(33)
		vid := TypeIDFromUint8(expected)
		got := Uint8FromTypeID(vid)
		require.Equal(t, expected, got)
		require.Equal(t, expected, vid.Uint8())
	}
	{
		m := map[TypeID]string{
			TypeIDFromSighash(Sighash(SIGHASH_GLOBAL_NAMESPACE, "hello")): "hello",
			TypeIDFromSighash(Sighash(SIGHASH_GLOBAL_NAMESPACE, "world")): "world",
		}

		expected := "world"
		require.Equal(t,
			expected,
			m[TypeIDFromSighash(Sighash(SIGHASH_GLOBAL_NAMESPACE, "world"))],
		)
	}
}

type Forest struct {
	T Tree
}

type Tree struct {
	Padding   [5]byte
	NodeCount uint32 `bin:"sizeof=Nodes"`
	Random    uint64
	Nodes     []*Node
}

var NodeVariantDef = NewVariantDefinition(
	Uint32TypeIDEncoding,

	[]VariantType{
		{"left_node", (*NodeLeft)(nil)},
		{"right_node", (*NodeRight)(nil)},
		{"inner_node", (*NodeInner)(nil)},
	})

type Node struct {
	BaseVariant
}

type NodeLeft struct {
	Key         uint32
	Description string
}

type NodeRight struct {
	Owner    uint64
	Padding  [2]byte
	Quantity Uint64
}

type NodeInner struct {
	Key Uint128
}

func (n *Node) UnmarshalWithDecoder(decoder *Decoder) error {
	return n.BaseVariant.UnmarshalBinaryVariant(decoder, NodeVariantDef)
}

func (n *Node) MarshalWithEncoder(encoder *Encoder) error {
	err := encoder.WriteUint32(n.TypeID.Uint32(), binary.LittleEndian)
	if err != nil {
		return err
	}
	return encoder.Encode(n.Impl)
}

func TestDecode_Variant(t *testing.T) {
	buf := []byte{
		0x73, 0x65, 0x72, 0x75, 0x6d, // Padding[5]byte
		0x05, 0x00, 0x00, 0x00, // Node length 5
		0xff, 0xff, 0x00, 0x00, 0x00, 0x0, 0x00, 0x00, // ROOT  65,535
		0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x61, 0x62, 0x63, // left node -> key = 3, description "abc"
		0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // right node -> owner = 3, quantity 13
		0x01, 0x00, 0x00, 0x00, 0x52, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x9b, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // right node -> owner = 82, quantity 923
		0x02, 0x00, 0x00, 0x00, 0xff, 0x7f, 0xc6, 0xa4, 0x7e, 0x8d, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // inner node -> key = 999999999999999
		0x02, 0x00, 0x00, 0x00, 0x23, 0xd3, 0xd8, 0x9a, 0x99, 0x7e, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // inner node -> key = 983623123129123
	}

	decoder := NewBinDecoder(buf)
	forest := Forest{}
	err := decoder.Decode(&forest)
	require.NoError(t, err)
	require.Equal(t, 0, decoder.Remaining())
	assert.Equal(t, Tree{
		Padding:   [5]byte{0x73, 0x65, 0x72, 0x75, 0x6d},
		NodeCount: 5,
		Random:    65535,
		Nodes: []*Node{
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(0, binary.LittleEndian),
					Impl: &NodeLeft{
						Key:         3,
						Description: "abc",
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(1, binary.LittleEndian),
					Impl: &NodeRight{
						Owner:    3,
						Padding:  [2]byte{0x00, 0x00},
						Quantity: 13,
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(1, binary.LittleEndian),
					Impl: &NodeRight{
						Owner:    82,
						Padding:  [2]byte{0x00, 0x00},
						Quantity: 923,
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(2, binary.LittleEndian),
					Impl: &NodeInner{
						Key: Uint128{
							Lo: 999999999999999,
							Hi: 0,
						},
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(2, binary.LittleEndian),
					Impl: &NodeInner{
						Key: Uint128{
							Lo: 983623123129123,
							Hi: 0,
						},
					},
				},
			},
		},
	}, forest.T)
}

func TestEncode_Variant(t *testing.T) {
	expectBuf := []byte{
		0x73, 0x65, 0x72, 0x75, 0x6d, // Padding[5]byte
		0x05, 0x00, 0x00, 0x00, // Node length 5
		0xff, 0xff, 0x00, 0x00, 0x00, 0x0, 0x00, 0x00, // ROOT  65,535
		0x00, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x61, 0x62, 0x63, // left node -> key = 3, description "abc"
		0x01, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // right node -> owner = 3, quantity 13
		0x01, 0x00, 0x00, 0x00, 0x52, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x9b, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // right node -> owner = 82, quantity 923
		0x02, 0x00, 0x00, 0x00, 0xff, 0x7f, 0xc6, 0xa4, 0x7e, 0x8d, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // inner node -> key = 999999999999999
		0x02, 0x00, 0x00, 0x00, 0x23, 0xd3, 0xd8, 0x9a, 0x99, 0x7e, 0x03, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, // inner node -> key = 983623123129123
	}

	buf := new(bytes.Buffer)
	enc := NewBinEncoder(buf)

	enc.Encode(&Forest{T: Tree{
		Padding:   [5]byte{0x73, 0x65, 0x72, 0x75, 0x6d},
		NodeCount: 5,
		Random:    65535,
		Nodes: []*Node{
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(0, binary.LittleEndian),
					Impl: &NodeLeft{
						Key:         3,
						Description: "abc",
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(1, binary.LittleEndian),
					Impl: &NodeRight{
						Owner:    3,
						Padding:  [2]byte{0x00, 0x00},
						Quantity: 13,
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(1, binary.LittleEndian),
					Impl: &NodeRight{
						Owner:    82,
						Padding:  [2]byte{0x00, 0x00},
						Quantity: 923,
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(2, binary.LittleEndian),
					Impl: &NodeInner{
						Key: Uint128{
							Lo: 999999999999999,
							Hi: 0,
						},
					},
				},
			},
			{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(2, binary.LittleEndian),
					Impl: &NodeInner{
						Key: Uint128{
							Lo: 983623123129123,
							Hi: 0,
						},
					},
				},
			},
		},
	}})

	assert.Equal(t, expectBuf, buf.Bytes())
}

type unexportesStruct struct {
	value uint32
}

func TestDecode_UnexporterStruct(t *testing.T) {
	buf := []byte{
		0x05, 0x00, 0x00, 0x00,
	}

	decoder := NewBinDecoder(buf)
	s := unexportesStruct{}
	err := decoder.Decode(&s)
	require.NoError(t, err)
	require.Equal(t, 4, decoder.Remaining())
	assert.Equal(t, unexportesStruct{value: 0}, s)
}

func TestEncode_UnexporterStruct(t *testing.T) {
	var expectData []byte

	buf := new(bytes.Buffer)
	enc := NewBinEncoder(buf)

	enc.Encode(&unexportesStruct{value: 5})
	assert.Equal(t, expectData, buf.Bytes())
}

// Test data setup helpers
func createSampleForest(numNodes int) *Forest {
	nodes := make([]*Node, numNodes)

	// Create a repeating pattern of different node types
	for i := 0; i < numNodes; i++ {
		switch i % 3 {
		case 0:
			nodes[i] = &Node{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(0, binary.LittleEndian),
					Impl: &NodeLeft{
						Key:         uint32(i),
						Description: "test description",
					},
				},
			}
		case 1:
			nodes[i] = &Node{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(1, binary.LittleEndian),
					Impl: &NodeRight{
						Owner:    uint64(i),
						Padding:  [2]byte{0x00, 0x00},
						Quantity: Uint64(i * 100),
					},
				},
			}
		case 2:
			nodes[i] = &Node{
				BaseVariant: BaseVariant{
					TypeID: TypeIDFromUint32(2, binary.LittleEndian),
					Impl: &NodeInner{
						Key: Uint128{
							Lo: uint64(i * 1000000),
							Hi: 0,
						},
					},
				},
			}
		}
	}

	return &Forest{
		T: Tree{
			Padding:   [5]byte{0x73, 0x65, 0x72, 0x75, 0x6d},
			NodeCount: uint32(numNodes),
			Random:    65535,
			Nodes:     nodes,
		},
	}
}

// Benchmarks for encoding
func BenchmarkEncode(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}

	for _, size := range sizes {
		b.Run("Forest/Nodes="+string(rune(size)), func(b *testing.B) {
			forest := createSampleForest(size)
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				buf := new(bytes.Buffer)
				enc := NewBinEncoder(buf)
				err := enc.Encode(forest)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmarks for decoding
func BenchmarkDecode(b *testing.B) {
	sizes := []int{1, 10, 100, 1000}

	for _, size := range sizes {
		// First create the encoded data
		forest := createSampleForest(size)
		buf := new(bytes.Buffer)
		enc := NewBinEncoder(buf)
		err := enc.Encode(forest)
		if err != nil {
			b.Fatal(err)
		}
		encodedData := buf.Bytes()

		b.Run("Forest/Nodes="+string(rune(size)), func(b *testing.B) {
			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				decoder := NewBinDecoder(encodedData)
				result := &Forest{}
				err := decoder.Decode(result)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	}
}

// Benchmark individual variant operations
func BenchmarkVariantOperations(b *testing.B) {
	// Test type ID creation
	b.Run("TypeIDFromUint32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = TypeIDFromUint32(uint32(i), binary.LittleEndian)
		}
	})

	b.Run("TypeIDFromUvarint32", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_ = TypeIDFromUvarint32(uint32(i))
		}
	})

	// Test variant definition creation
	b.Run("NewVariantDefinition", func(b *testing.B) {
		types := []VariantType{
			{"left_node", (*NodeLeft)(nil)},
			{"right_node", (*NodeRight)(nil)},
			{"inner_node", (*NodeInner)(nil)},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = NewVariantDefinition(Uint32TypeIDEncoding, types)
		}
	})
}

// Benchmark variant marshaling/unmarshaling with different node types
func BenchmarkVariantNodes(b *testing.B) {
	// Test left node
	b.Run("LeftNode/Marshal", func(b *testing.B) {
		node := &Node{
			BaseVariant: BaseVariant{
				TypeID: TypeIDFromUint32(0, binary.LittleEndian),
				Impl: &NodeLeft{
					Key:         42,
					Description: "test",
				},
			},
		}

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			buf := new(bytes.Buffer)
			enc := NewBinEncoder(buf)
			err := node.MarshalWithEncoder(enc)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	// Create encoded left node for unmarshal testing
	leftNode := &Node{
		BaseVariant: BaseVariant{
			TypeID: TypeIDFromUint32(0, binary.LittleEndian),
			Impl: &NodeLeft{
				Key:         42,
				Description: "test",
			},
		},
	}
	buf := new(bytes.Buffer)
	enc := NewBinEncoder(buf)
	err := leftNode.MarshalWithEncoder(enc)
	if err != nil {
		b.Fatal(err)
	}
	encodedLeftNode := buf.Bytes()

	b.Run("LeftNode/Unmarshal", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			decoder := NewBinDecoder(encodedLeftNode)
			node := &Node{}
			err := node.UnmarshalWithDecoder(decoder)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Benchmark concurrent access patterns
func BenchmarkConcurrentVariant(b *testing.B) {
	b.Run("Parallel/Encode", func(b *testing.B) {
		forest := createSampleForest(100)
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				buf := new(bytes.Buffer)
				enc := NewBinEncoder(buf)
				err := enc.Encode(forest)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})

	// Create encoded data for parallel decode testing
	forest := createSampleForest(100)
	buf := new(bytes.Buffer)
	enc := NewBinEncoder(buf)
	err := enc.Encode(forest)
	if err != nil {
		b.Fatal(err)
	}
	encodedData := buf.Bytes()

	b.Run("Parallel/Decode", func(b *testing.B) {
		b.ResetTimer()

		b.RunParallel(func(pb *testing.PB) {
			for pb.Next() {
				decoder := NewBinDecoder(encodedData)
				result := &Forest{}
				err := decoder.Decode(result)
				if err != nil {
					b.Fatal(err)
				}
			}
		})
	})
}
