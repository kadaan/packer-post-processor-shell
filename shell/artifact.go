package shell

import (
	"fmt"
	"os"
)

type Artifact struct {
	Path     string
	BuilderId string
	Provider string
}

func NewArtifact(provider, builderId string, path string) *Artifact {
	return &Artifact{
		Path:     path,
		Provider: provider,
	}
}

func (*Artifact) BuilderId() string {
	return a.BuilderId
}

func (a *Artifact) Files() []string {
	return []string{a.Path}
}

func (a *Artifact) Id() string {
	return a.Provider
}

func (a *Artifact) String() string {
	return fmt.Sprintf("'%s' provider box: %s", a.Provider, a.Path)
}

func (a *Artifact) State(name string) interface{} {
	return nil
}

func (a *Artifact) Destroy() error {
	return os.Remove(a.Path)
}
