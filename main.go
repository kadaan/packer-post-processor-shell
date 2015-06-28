package main

import (
	"github.com/mitchellh/packer/packer/plugin"
	"github.com/kadaan/packer-post-processor-shell/shell"
)

func main() {
	server, err := plugin.Server()
	if err != nil {
		panic(err)
	}
	server.RegisterPostProcessor(new(shell.ShellPostProcessor))
	server.Serve()
}
