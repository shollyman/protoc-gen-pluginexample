// Copyright 2024 Google LLC
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

// protoc-gen-pluginexample is a protoc plugin that demonstrates plugin development.
package main

import (
	"bytes"
	"fmt"
	"io"
	"log"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {

	// First, read bytes from stdin
	b, err := io.ReadAll(os.Stdin)
	if err != nil {
		log.Fatal(err)
	}

	// unmarshal bytes as a code generator request
	var req pluginpb.CodeGeneratorRequest
	if err := proto.Unmarshal(b, &req); err != nil {
		log.Fatalf("Unmarshal: %v", err)
	}

	// Here's where the magic actually happens
	resp, err := processRequest(&req)
	if err != nil {
		log.Fatalf("processRequest: %v", err)
	}

	// convert the response to bytes and report it to stdout.
	outBytes, err := proto.Marshal(resp)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := os.Stdout.Write(outBytes); err != nil {
		log.Fatal(err)
	}
}

// processRequest is the core of the plugin.
func processRequest(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse, error) {
	resp := &pluginpb.CodeGeneratorResponse{
		// communicate this plugin understands proto3 optional.
		SupportedFeatures: proto.Uint64(uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)),
	}

	// first, produce the request as a json document.
	f, err := recordRequest(req)
	if err != nil {
		return nil, fmt.Errorf("recordRequest failed: %w", err)
	}
	resp.File = append(resp.File, f)

	// now, walk the contents of the request to gather basic stats
	f, err = recordStats(req)
	if err != nil {
		return nil, fmt.Errorf("recordRequest failed: %w", err)
	}
	resp.File = append(resp.File, f)

	// now, walk the produce an entity graph in graphviz dotfile format.
	f, err = generateGraph(req)
	if err != nil {
		return nil, fmt.Errorf("recordRequest failed: %w", err)
	}
	resp.File = append(resp.File, f)

	// return the response
	return resp, nil
}

// recordRequest constructs a File entity the contains the JSON-formatted contents
// of the incoming request.
func recordRequest(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse_File, error) {
	jsonBytes := protojson.Format(req)
	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("request_dump.json"),
		Content: proto.String(string(jsonBytes)),
	}, nil
}

// recordStats demonstrates walking the request to collect basic stats about the descriptor types present.
func recordStats(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse_File, error) {
	stats := struct {
		NumFiles    int
		NumServices int
		NumMethods  int
		NumMessages int
		NumFields   int
	}{}

	for _, f := range req.GetProtoFile() {
		stats.NumFiles = stats.NumFiles + 1
		// get RPC service and method starts
		for _, srv := range f.GetService() {
			stats.NumServices = stats.NumServices + 1
			stats.NumMethods = stats.NumMethods + len(srv.GetMethod())
		}
		for _, msg := range f.GetMessageType() {
			// note: this doesn't correctly attribute nested messages (messages defined inside another message)
			stats.NumMessages = stats.NumMessages + 1
			stats.NumFields = stats.NumFields + len(msg.GetField())
		}
	}

	buf := new(bytes.Buffer)
	fmt.Fprintln(buf, "stats for code generation request")
	fmt.Fprintf(buf, "num files: %d\n", stats.NumFiles)
	fmt.Fprintf(buf, "num services: %d\n", stats.NumServices)
	fmt.Fprintf(buf, "num methods: %d\n", stats.NumMethods)
	fmt.Fprintf(buf, "num messages: %d\n", stats.NumMessages)
	fmt.Fprintf(buf, "num fields: %d\n", stats.NumFields)

	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("request_stats.txt"),
		Content: proto.String(buf.String()),
	}, nil
}

// generateGraph is a very naive attempt to produce an entity graph for the provided request.
// It produces a dot file, which can be used by graphviz to produce an image.
func generateGraph(req *pluginpb.CodeGeneratorRequest) (*pluginpb.CodeGeneratorResponse_File, error) {
	nodeBuf := new(bytes.Buffer)
	vertexBuf := new(bytes.Buffer)
	// First, add RPC entries
	for _, f := range req.GetProtoFile() {
		for _, srv := range f.GetService() {
			qService := fmt.Sprintf(".%s.%s", f.GetPackage(), srv.GetName())
			// write service node
			fmt.Fprintf(nodeBuf, "%q [shape=diamond]\n", qService)
			for _, meth := range srv.GetMethod() {
				qName := fmt.Sprintf("%s.%s", qService, meth.GetName())
				// write method node info
				fmt.Fprintf(nodeBuf, "%q [shape=circle]\n", qName)
				// write link info
				fmt.Fprintf(vertexBuf, "%q -> %q [style=dashed]\n", qService, qName)
				fmt.Fprintf(vertexBuf, "%q -> %q [style=dashed, color=red]\n", qName, meth.GetInputType())
				fmt.Fprintf(vertexBuf, "%q -> %q [style=dashed, color=blue]\n", qName, meth.GetOutputType())
			}
		}

		// Now, build message graph
		for _, m := range f.GetMessageType() {
			generateGraphMessages(m, fmt.Sprintf(".%s", f.GetPackage()), nodeBuf, vertexBuf)
		}
	}

	return &pluginpb.CodeGeneratorResponse_File{
		Name:    proto.String("entity_graph.dot"),
		Content: proto.String(fmt.Sprintf("digraph entities {\n\n%s\n%s\n}", nodeBuf.String(), vertexBuf.String())),
	}, nil
}

func generateGraphMessages(dp *descriptorpb.DescriptorProto, prefix string, nbuf io.Writer, vbuf io.Writer) {
	qName := fmt.Sprintf("%s.%s", prefix, dp.GetName())
	fmt.Fprintf(nbuf, "%q [shape=square]\n", qName)
	for _, field := range dp.GetField() {
		if field.GetType() == descriptorpb.FieldDescriptorProto_TYPE_MESSAGE &&
			field.GetTypeName() != "" {
			fmt.Fprintf(vbuf, "%q -> %q\n", qName, field.GetTypeName())
		}
	}
	for _, child := range dp.GetNestedType() {
		generateGraphMessages(child, qName, nbuf, vbuf)
	}
}
