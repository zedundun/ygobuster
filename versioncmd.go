package main

import (
	"context"
	"flag"
	"fmt"

	"io/ioutil"

	"github.com/google/subcommands"
)

type versionCmd struct {
}

func (p *versionCmd) Name() string {
	return "version"
}

func (p *versionCmd) Synopsis() string {
	return "print version"
}

func (p *versionCmd) Usage() string {
	return "print version\n" //output for -h
}

func (p *versionCmd) SetFlags(*flag.FlagSet) {
}

func (p *versionCmd) Execute(_ context.Context, f *flag.FlagSet, _ ...interface{}) subcommands.ExitStatus {
	txt, _ := ioutil.ReadFile("/ygobuster/VERSION")
	fmt.Println(string(txt))
	return subcommands.ExitSuccess
}
