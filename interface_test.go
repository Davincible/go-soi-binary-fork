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
	"testing"

	"github.com/stretchr/testify/assert"
)

type Example struct {
	Prefix byte
	Value  uint32
}

func (e *Example) UnmarshalWithDecoder(decoder *Decoder) (err error) {
	if e.Prefix, err = decoder.ReadByte(); err != nil {
		return err
	}
	if e.Value, err = decoder.ReadUint32(BE()); err != nil {
		return err
	}
	return nil
}

func (e *Example) MarshalWithEncoder(encoder *Encoder) error {
	if err := encoder.WriteByte(e.Prefix); err != nil {
		return err
	}
	return encoder.WriteUint32(e.Value, BE())
}

func TestMarshalWithEncoder(t *testing.T) {
	buf := new(bytes.Buffer)
	e := &Example{Value: 72, Prefix: 0xaa}
	enc := NewBinEncoder(buf)
	enc.Encode(e)

	assert.Equal(t, []byte{
		0xaa, 0x00, 0x00, 0x00, 0x48,
	}, buf.Bytes())
}

func TestUnmarshalWithDecoder(t *testing.T) {
	buf := []byte{
		0xaa, 0x00, 0x00, 0x00, 0x48,
	}

	e := &Example{}
	d := NewBinDecoder(buf)
	err := d.Decode(e)
	assert.NoError(t, err)
	assert.Equal(t, e, &Example{Value: 72, Prefix: 0xaa})
	assert.Equal(t, 0, d.Remaining())
}
