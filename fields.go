package struc

import (
	"encoding/binary"
	"io"
	"reflect"
	"strings"
)

type Fields []*Field

func (f Fields) SetByteOrder(order binary.ByteOrder) {
	for _, field := range f {
		field.Order = order
	}
}

func (f Fields) String() string {
	fields := make([]string, len(f))
	for i, field := range f {
		fields[i] = field.String()
	}
	return "{" + strings.Join(fields, ", ") + "}"
}

func (f Fields) Sizeof(val reflect.Value) int {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	size := 0
	for i, field := range f {
		v := val.Field(i)
		if v.CanSet() {
			size += field.Size(v)
		}
	}
	return size
}

func (f Fields) Len(val reflect.Value) int {
	k := val.Kind()
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	switch k {
		case reflect.Array:
			fallthrough
		case reflect.Chan:
			fallthrough
		case reflect.Map:
			fallthrough
		case reflect.Slice:
			fallthrough
		case reflect.String:
			return val.Len()
		case reflect.Struct:
			size := 0
			for i := 0; i < val.NumField(); i++  {
				v := val.Field(i)
				flen := f.Len(v)
				size += flen
			}
			return size
		default:
			return int(val.Type().Size())
	}
	panic("Shouldn't reach here")
}

func (f Fields) Pack(buf []byte, val reflect.Value) error {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	pos := 0
	for i, field := range f {
		if !field.CanSet {
			continue
		}
		v := val.Field(i)
		length := field.Len
		if field.Sizefrom != nil {
			length = int(val.FieldByIndex(field.Sizefrom).Int())
		}
		if length <= 0 && field.Slice {
			length = field.Size(v)
		}
		if field.Sizeof != nil {
			length = f.Len(val.FieldByIndex(field.Sizeof))
			v = reflect.ValueOf(length)
		}
		err := field.Pack(buf[pos:], v, length)
		if err != nil {
			return err
		}
		pos += field.Size(v)
	}
	return nil
}

func (f Fields) Unpack(r io.Reader, val reflect.Value) error {
	for val.Kind() == reflect.Ptr {
		val = val.Elem()
	}
	var tmp [8]byte
	var buf []byte
	for i, field := range f {
		if !field.CanSet {
			continue
		}
		v := val.Field(i)
		length := field.Len
		if field.Sizefrom != nil {
			length = int(val.FieldByIndex(field.Sizefrom).Int())
		}
		if v.Kind() == reflect.Ptr && !v.Elem().IsValid() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if field.Type == Struct {
			fields, err := parseFields(v)
			if err != nil {
				return err
			}
			if err := fields.Unpack(r, v); err != nil {
				return err
			}
			continue
		} else {
			size := length * field.Type.Size()
			if size < 8 {
				buf = tmp[:size]
			} else {
				buf = make([]byte, size)
			}
			if _, err := io.ReadFull(r, buf); err != nil {
				return err
			}
			err := field.Unpack(buf[:size], v, length)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
