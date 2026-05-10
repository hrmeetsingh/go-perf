package cmd

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/hrmeetsingh/go-perf/internal/payload"
	protolib "github.com/hrmeetsingh/go-perf/internal/proto"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate",
	Short: "Generate sample payloads from proto files",
	Long:  "Parse a proto file and generate sample JSON payloads for the specified RPC method.",
	RunE:  runGenerate,
}

var (
	genProto   string
	genService string
	genMethod  string
	genOutput  string
)

func init() {
	generateCmd.Flags().StringVar(&genProto, "proto", "", "path to proto file")
	generateCmd.Flags().StringVar(&genService, "service", "", "fully qualified service name")
	generateCmd.Flags().StringVar(&genMethod, "method", "", "method name")
	generateCmd.Flags().StringVar(&genOutput, "output", "", "output file (default: stdout)")

	generateCmd.MarkFlagRequired("proto")
	generateCmd.MarkFlagRequired("service")
	generateCmd.MarkFlagRequired("method")

	rootCmd.AddCommand(generateCmd)
}

func runGenerate(cmd *cobra.Command, args []string) error {
	fd, err := protolib.ParseProtoFile(genProto)
	if err != nil {
		return fmt.Errorf("parsing proto file: %w", err)
	}

	fields, err := protolib.GetInputFields(fd, genService, genMethod)
	if err != nil {
		return fmt.Errorf("getting input fields: %w", err)
	}

	descriptors := protoFieldsToPayloadFields(fields)
	sample := payload.GenerateSamplePayload(descriptors)

	data, err := json.MarshalIndent(sample, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling payload: %w", err)
	}

	if genOutput != "" {
		if err := os.WriteFile(genOutput, data, 0644); err != nil {
			return fmt.Errorf("writing output: %w", err)
		}
		fmt.Fprintf(os.Stderr, "Payload written to %s\n", genOutput)
	} else {
		fmt.Println(string(data))
	}

	return nil
}

func protoFieldsToPayloadFields(fields []protolib.FieldInfo) []payload.FieldDescriptor {
	result := make([]payload.FieldDescriptor, len(fields))
	for i, f := range fields {
		fd := payload.FieldDescriptor{
			Name: f.Name,
			Type: payload.FieldType(protolib.MapFieldType(f.Type)),
			Path: f.FullPath,
		}
		if len(f.Children) > 0 {
			fd.Type = payload.FieldTypeMessage
			fd.Children = protoFieldsToPayloadFields(f.Children)
		}
		result[i] = fd
	}
	return result
}
