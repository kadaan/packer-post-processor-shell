package main

import "github.com/mitchellh/packer/packer"

type Artifact struct {
	builderId string
	files []string
	id string
	str string
}

func NewArtifact(artifact packer.Artifact) *Artifact {
	return &Artifact{
		builderId:     artifact.BuilderId(),
		files:     artifact.Files(),
		id:     artifact.Id(),
		str:     artifact.String(),
	}
}

func (a *Artifact) BuilderId() string {
	return a.builderId
}

func (a *Artifact) Files() []string {
	return a.files
}

func (a *Artifact) Id() string {
	return a.id
}

func (a *Artifact) String() string {
	return a.str;
}

func (a *Artifact) State(name string) interface{} {
	return nil
}

func (a *Artifact) Destroy() error {
	return nil
}
