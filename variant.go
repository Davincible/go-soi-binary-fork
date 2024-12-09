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
	"fmt"
	"reflect"
	"strings"
)

// Variant defines the interface for types that can be used in variant implementations.
// It emulates the `fc::static_variant` type from C++.
type Variant interface {
	// Assign sets the type ID and implementation for the variant.
	Assign(typeID TypeID, impl interface{})

	// Obtain retrieves the current type ID, type name, and implementation from the variant.
	// The VariantDefinition is used to look up the type name from the type ID.
	Obtain(*VariantDefinition) (typeID TypeID, typeName string, impl interface{})
}

// VariantType represents a single type definition within a variant.
// It contains both the name of the type and an example instance of the type
// used for reflection.
type VariantType struct {
	Name string      // The name of the type
	Type interface{} // An instance of the type for reflection
}

// CachedTypeInfo holds pre-computed reflection information for a type to optimize
// performance by avoiding repeated reflection operations during unmarshaling.
type CachedTypeInfo struct {
	Type         reflect.Type       // The full reflection type information
	Name         string             // The name of the type
	IsPtr        bool               // Whether the type is a pointer type
	ElementType  reflect.Type       // For pointer types, the type being pointed to
	ZeroValue    reflect.Value      // Pre-computed zero value for the type
	NewValueFunc func() interface{} // Optimized function to create new instances
}

// VariantDefinition defines the complete structure of a variant type, including
// mappings between type IDs, names, and implementations, as well as cached
// reflection information for performance optimization.
type VariantDefinition struct {
	typeIDToCache  map[TypeID]*CachedTypeInfo // Maps type IDs to cached type information
	typeIDToName   map[TypeID]string          // Maps type IDs to type names
	typeNameToID   map[string]TypeID          // Maps type names to type IDs
	typeIDEncoding TypeIDEncoding             // The encoding method for type IDs
}

// TypeID defines the internal representation of an instruction type ID
// (or account type, etc. in anchor programs)
// and it's used to associate instructions to decoders in the variant tracker.
type TypeID [8]byte

// Bytes returns the TypeID as a byte slice.
func (vid TypeID) Bytes() []byte {
	return vid[:]
}

// Uvarint32 parses the TypeID to a uint32 using variable-length encoding.
func (vid TypeID) Uvarint32() uint32 {
	return Uvarint32FromTypeID(vid)
}

// Uint32 parses the TypeID to a uint32 using the specified byte order.
func (vid TypeID) Uint32() uint32 {
	return Uint32FromTypeID(vid, binary.LittleEndian)
}

// Uint8 parses the TypeID to a uint8.
func (vid TypeID) Uint8() uint8 {
	return Uint8FromTypeID(vid)
}

// Equal returns true if the provided bytes are equal to
// the bytes of the TypeID.
func (vid TypeID) Equal(b []byte) bool {
	return bytes.Equal(vid.Bytes(), b)
}

// TypeIDFromBytes converts a []byte to a TypeID.
// The provided slice must be 8 bytes long or less.
func TypeIDFromBytes(slice []byte) (id TypeID) {
	copy(id[:], slice)
	return id
}

// TypeIDFromSighash converts a sighash bytes to a TypeID.
func TypeIDFromSighash(sh []byte) TypeID {
	return TypeIDFromBytes(sh)
}

// TypeIDFromUvarint32 converts a Uvarint to a TypeID.
func TypeIDFromUvarint32(v uint32) TypeID {
	buf := make([]byte, 8)
	l := binary.PutUvarint(buf, uint64(v))
	return TypeIDFromBytes(buf[:l])
}

// TypeIDFromUint32 converts a uint32 to a TypeID using the specified byte order.
func TypeIDFromUint32(v uint32, bo binary.ByteOrder) TypeID {
	out := make([]byte, TypeSize.Uint32)
	bo.PutUint32(out, v)
	return TypeIDFromBytes(out)
}

// TypeIDFromUint8 converts a uint8 to a TypeID.
func TypeIDFromUint8(v uint8) TypeID {
	return TypeIDFromBytes([]byte{v})
}

// Uvarint32FromTypeID parses a TypeID bytes to a uvarint32.
func Uvarint32FromTypeID(vid TypeID) (out uint32) {
	l, _ := binary.Uvarint(vid[:])
	out = uint32(l)
	return out
}

// Uint32FromTypeID parses a TypeID bytes to a uint32 using the specified byte order.
func Uint32FromTypeID(vid TypeID, order binary.ByteOrder) (out uint32) {
	out = order.Uint32(vid[:])
	return out
}

// Uint8FromTypeID parses a TypeID bytes to a uint8.
func Uint8FromTypeID(vid TypeID) (out uint8) {
	return vid[0]
}

// TypeIDEncoding defines how type IDs are encoded in the binary format.
type TypeIDEncoding uint32

const (
	// Uvarint32TypeIDEncoding uses variable-length uint32 encoding
	Uvarint32TypeIDEncoding TypeIDEncoding = iota
	// Uint32TypeIDEncoding uses fixed-length uint32 encoding
	Uint32TypeIDEncoding
	// Uint8TypeIDEncoding uses uint8 encoding
	Uint8TypeIDEncoding
	// AnchorTypeIDEncoding uses instruction sighash encoding (Anchor SDK)
	AnchorTypeIDEncoding
	// NoTypeIDEncoding is used when there's only one variant per program
	NoTypeIDEncoding
)

// NoTypeIDDefaultID is the default TypeID used when NoTypeIDEncoding is specified
var NoTypeIDDefaultID = TypeIDFromUint8(0)

// NewVariantDefinition creates a variant definition based on the provided types.
// For anchor instructions, the name defines the binary variant value.
// For all other types, the ordering defines the binary variant value.
// The types must be passed in the correct order to match the binary format.
func NewVariantDefinition(typeIDEncoding TypeIDEncoding, types []VariantType) *VariantDefinition {
	typeCount := len(types)
	if typeCount < 0 {
		panic("it's not valid to create a variant definition without any types")
	}

	out := &VariantDefinition{
		typeIDEncoding: typeIDEncoding,
		typeIDToCache:  make(map[TypeID]*CachedTypeInfo, typeCount),
		typeIDToName:   make(map[TypeID]string, typeCount),
		typeNameToID:   make(map[string]TypeID, typeCount),
	}

	for i, typeDef := range types {
		var typeID TypeID
		switch typeIDEncoding {
		case Uvarint32TypeIDEncoding:
			typeID = TypeIDFromUvarint32(uint32(i))
		case Uint32TypeIDEncoding:
			typeID = TypeIDFromUint32(uint32(i), binary.LittleEndian)
		case Uint8TypeIDEncoding:
			typeID = TypeIDFromUint8(uint8(i))
		case AnchorTypeIDEncoding:
			typeID = TypeIDFromSighash(Sighash(SIGHASH_GLOBAL_NAMESPACE, typeDef.Name))
		case NoTypeIDEncoding:
			if len(types) != 1 {
				panic(fmt.Sprintf("NoTypeIDEncoding can only have one variant type definition, got %v", len(types)))
			}
			typeID = NoTypeIDDefaultID
		default:
			panic(fmt.Errorf("unsupported TypeIDEncoding: %v", typeIDEncoding))
		}

		// Cache reflection information for better performance
		goType := reflect.TypeOf(typeDef.Type)
		cache := &CachedTypeInfo{
			Type:  goType,
			Name:  typeDef.Name,
			IsPtr: goType.Kind() == reflect.Ptr,
		}

		if cache.IsPtr {
			cache.ElementType = goType.Elem()
			cache.NewValueFunc = func() interface{} {
				return reflect.New(cache.ElementType).Interface()
			}
		} else {
			cache.ElementType = goType
			cache.ZeroValue = reflect.Zero(goType)
			cache.NewValueFunc = func() interface{} {
				return reflect.New(goType).Interface()
			}
		}

		out.typeIDToCache[typeID] = cache
		out.typeIDToName[typeID] = typeDef.Name
		out.typeNameToID[typeDef.Name] = typeID
	}

	return out
}

// TypeID returns the TypeID for a given type name.
// Panics if the type name is not found in the definition.
func (d *VariantDefinition) TypeID(name string) TypeID {
	id, found := d.typeNameToID[name]
	if !found {
		knownNames := make([]string, len(d.typeNameToID))
		i := 0
		for name := range d.typeNameToID {
			knownNames[i] = name
			i++
		}

		panic(fmt.Errorf("trying to use an unknown type name %q, known names are %q", name, strings.Join(knownNames, ", ")))
	}

	return id
}

// BaseVariant provides a basic implementation of the Variant interface.
type BaseVariant struct {
	TypeID TypeID      // The type identifier
	Impl   interface{} // The actual implementation
}

// Ensure BaseVariant implements the Variant interface
var _ Variant = &BaseVariant{}

// Assign implements the Variant interface by setting the type ID and implementation.
func (a *BaseVariant) Assign(typeID TypeID, impl interface{}) {
	a.TypeID = typeID
	a.Impl = impl
}

// Obtain implements the Variant interface by returning the current type information.
func (a *BaseVariant) Obtain(def *VariantDefinition) (typeID TypeID, typeName string, impl interface{}) {
	return a.TypeID, def.typeIDToName[a.TypeID], a.Impl
}

// UnmarshalBinaryVariant decodes a binary-encoded variant using the provided definition.
// It handles different type ID encodings and uses cached type information for efficient
// instantiation and decoding of the variant implementation.
func (a *BaseVariant) UnmarshalBinaryVariant(decoder *Decoder, def *VariantDefinition) error {
	var typeID TypeID
	var err error

	switch def.typeIDEncoding {
	case NoTypeIDEncoding:
		typeID = NoTypeIDDefaultID
	case Uvarint32TypeIDEncoding:
		val, err := decoder.ReadUvarint32()
		if err != nil {
			return fmt.Errorf("uvarint32: unable to read variant type id: %s", err)
		}
		typeID = TypeIDFromUvarint32(val)
	case Uint32TypeIDEncoding:
		val, err := decoder.ReadUint32(binary.LittleEndian)
		if err != nil {
			return fmt.Errorf("uint32: unable to read variant type id: %s", err)
		}
		typeID = TypeIDFromUint32(val, binary.LittleEndian)
	case Uint8TypeIDEncoding:
		id, err := decoder.ReadUint8()
		if err != nil {
			return fmt.Errorf("uint8: unable to read variant type id: %s", err)
		}
		typeID = TypeIDFromBytes([]byte{id})
	case AnchorTypeIDEncoding:
		typeID, err = decoder.ReadTypeID()
		if err != nil {
			return fmt.Errorf("anchor: unable to read variant type id: %s", err)
		}
	}

	a.TypeID = typeID

	cache := def.typeIDToCache[typeID]
	if cache == nil {
		return fmt.Errorf("no known type for type id %v", typeID)
	}

	// Use pre-computed type information to create and decode the value
	impl := cache.NewValueFunc()
	if err = decoder.Decode(impl); err != nil {
		return fmt.Errorf("unable to decode variant type %d: %s", typeID, err)
	}

	// For non-pointer types, extract the value from the pointer we created for decoding
	if !cache.IsPtr {
		a.Impl = reflect.ValueOf(impl).Elem().Interface()
	} else {
		a.Impl = impl
	}

	return nil
}
