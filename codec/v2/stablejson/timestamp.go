package stablejson

import (
	"strings"

	"google.golang.org/protobuf/reflect/protorange"
	"google.golang.org/protobuf/reflect/protoreflect"
)

func marshalTimestamp(writer *strings.Builder, message protoreflect.Message) error {
	return protorange.Break
}
