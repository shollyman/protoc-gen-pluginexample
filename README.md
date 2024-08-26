# protoc-gen-pluginexample

This repo is a quick and dirty demonstrate around developing a protobuf plugin.

The simplest way to generate a go binary is to CD to this directory and invoke

## quick and dirty setup
```
$ cd <this directory>
$ go build
$ export PATH=$(pwd):$PATH
```

### some example invocations of the protoc compiler

generate go and python bindings
```
mkdir output_basic
protoc -go_out=output_basic -python_out=output_basic testdata/person.proto
```

invoke the test plugin
```
mkdir output_more
protoc -pluginexample_out=output_more testdata/person.proto
```

