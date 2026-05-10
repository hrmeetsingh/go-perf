package proto

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jhump/protoreflect/desc"
	"github.com/jhump/protoreflect/desc/protoparse"
)

type MethodInfo struct {
	Name           string
	FullName       string
	IsClientStream bool
	IsServerStream bool
	InputType      string
	OutputType     string
}

type FieldInfo struct {
	Name     string
	Type     string
	FullPath string
	Children []FieldInfo
}

func DiscoverProtoFiles(path string) ([]string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("accessing path %q: %w", path, err)
	}

	if !info.IsDir() {
		return []string{path}, nil
	}

	var files []string
	err = filepath.Walk(path, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() && strings.HasSuffix(p, ".proto") {
			files = append(files, p)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking directory %q: %w", path, err)
	}

	return files, nil
}

func ParseProtoFile(path string) (*desc.FileDescriptor, error) {
	dir := filepath.Dir(path)
	filename := filepath.Base(path)

	parser := protoparse.Parser{
		ImportPaths: []string{dir},
	}

	fds, err := parser.ParseFiles(filename)
	if err != nil {
		return nil, fmt.Errorf("parsing proto file %q: %w", path, err)
	}

	if len(fds) == 0 {
		return nil, fmt.Errorf("no file descriptors returned for %q", path)
	}

	return fds[0], nil
}

func ListServices(fd *desc.FileDescriptor) []string {
	services := fd.GetServices()
	names := make([]string, len(services))
	for i, svc := range services {
		names[i] = svc.GetFullyQualifiedName()
	}
	return names
}

func ListMethods(fd *desc.FileDescriptor, serviceFQN string) ([]MethodInfo, error) {
	svc := fd.FindSymbol(serviceFQN)
	if svc == nil {
		return nil, fmt.Errorf("service %q not found", serviceFQN)
	}

	svcDesc, ok := svc.(*desc.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service", serviceFQN)
	}

	methods := svcDesc.GetMethods()
	result := make([]MethodInfo, len(methods))
	for i, m := range methods {
		result[i] = MethodInfo{
			Name:           m.GetName(),
			FullName:       m.GetFullyQualifiedName(),
			IsClientStream: m.IsClientStreaming(),
			IsServerStream: m.IsServerStreaming(),
			InputType:      m.GetInputType().GetFullyQualifiedName(),
			OutputType:     m.GetOutputType().GetFullyQualifiedName(),
		}
	}
	return result, nil
}

func GetMethodInfo(fd *desc.FileDescriptor, serviceFQN, methodName string) (*MethodInfo, error) {
	methods, err := ListMethods(fd, serviceFQN)
	if err != nil {
		return nil, err
	}

	for _, m := range methods {
		if m.Name == methodName {
			return &m, nil
		}
	}
	return nil, fmt.Errorf("method %q not found in service %q", methodName, serviceFQN)
}

func GetInputFields(fd *desc.FileDescriptor, serviceFQN, methodName string) ([]FieldInfo, error) {
	svc := fd.FindSymbol(serviceFQN)
	if svc == nil {
		return nil, fmt.Errorf("service %q not found", serviceFQN)
	}

	svcDesc, ok := svc.(*desc.ServiceDescriptor)
	if !ok {
		return nil, fmt.Errorf("%q is not a service", serviceFQN)
	}

	var methodDesc *desc.MethodDescriptor
	for _, m := range svcDesc.GetMethods() {
		if m.GetName() == methodName {
			methodDesc = m
			break
		}
	}
	if methodDesc == nil {
		return nil, fmt.Errorf("method %q not found", methodName)
	}

	inputMsg := methodDesc.GetInputType()
	return extractFields(inputMsg, ""), nil
}

// MapFieldType converts a protobuf field type string (e.g. "TYPE_STRING")
// to the corresponding payload.FieldType constant name.
func MapFieldType(protoType string) string {
	switch protoType {
	case "TYPE_STRING":
		return "string"
	case "TYPE_INT32", "TYPE_SINT32", "TYPE_SFIXED32":
		return "int32"
	case "TYPE_INT64", "TYPE_SINT64", "TYPE_SFIXED64":
		return "int64"
	case "TYPE_BOOL":
		return "bool"
	case "TYPE_DOUBLE":
		return "double"
	case "TYPE_FLOAT":
		return "float"
	case "TYPE_MESSAGE":
		return "message"
	default:
		return "string"
	}
}

func extractFields(msg *desc.MessageDescriptor, prefix string) []FieldInfo {
	fields := msg.GetFields()
	result := make([]FieldInfo, 0, len(fields))

	for _, f := range fields {
		path := f.GetName()
		if prefix != "" {
			path = prefix + "." + f.GetName()
		}

		fi := FieldInfo{
			Name:     f.GetName(),
			Type:     f.GetType().String(),
			FullPath: path,
		}

		if f.GetMessageType() != nil {
			fi.Children = extractFields(f.GetMessageType(), path)
		}

		result = append(result, fi)
	}
	return result
}
