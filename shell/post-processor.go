package shell

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"strings"

	"github.com/mitchellh/packer/common"
	"github.com/mitchellh/packer/helper/config"
	"github.com/mitchellh/packer/packer"
	"github.com/mitchellh/packer/template/interpolate"
)

type Config struct {
	common.PackerConfig `mapstructure:",squash"`

	// An inline script to execute. Multiple strings are all executed
	// in the context of a single shell.
	Inline []string `mapstructure:"inline"`

	// The shebang value used when running inline scripts.
	InlineShebang string `mapstructure:"inline_shebang"`

	// The local path of the shell script to upload and execute.
	Script string `mapstructure:"script"`

	// An array of environment variables that will be injected before
	// your command(s) are executed.
	Vars []string `mapstructure:"environment_vars"`

	// An array of multiple scripts to run.
	Scripts []string `mapstructure:"scripts"`

	TargetPath string `mapstructure:"target"`

	KeepInputArtifact bool   `mapstructure:"keep_input_artifact"`

	ctx interpolate.Context
}

type ShellPostProcessor struct {
	config Config
}

type OutputPathTemplate struct {
	ArtifactId string
	BuildName  string
	Provider   string
}

func (p *ShellPostProcessor) Configure(raws ...interface{}) error {
	err := config.Decode(&p.config, &config.DecodeOpts{
		Interpolate:        true,
		InterpolateContext: &p.config.ctx,
		InterpolateFilter: &interpolate.RenderFilter{
			Exclude: []string{},
		},
	}, raws...)
	if err != nil {
		return err
	}

	errs := new(packer.MultiError)

	if p.config.InlineShebang == "" {
		p.config.InlineShebang = "/bin/sh"
	}

	if p.config.Scripts == nil {
		p.config.Scripts = make([]string, 0)
	}

	if p.config.Vars == nil {
		p.config.Vars = make([]string, 0)
	}

	if p.config.Script != "" && len(p.config.Scripts) > 0 {
		errs = packer.MultiErrorAppend(errs,
			errors.New("Only one of script or scripts can be specified."))
	}

	if p.config.Script != "" {
		p.config.Scripts = []string{p.config.Script}
	}

	if err = interpolate.Validate(p.config.TargetPath, &p.config.ctx); err != nil {
		errs = packer.MultiErrorAppend(
			errs, fmt.Errorf("Error parsing target template: %s", err))
	}

	templates := map[string]*string{
		"inline_shebang": &p.config.InlineShebang,
		"script":         &p.config.Script,
	}

	for n, ptr := range templates {
		*ptr, err = interpolate.Render(*ptr, &p.config.ctx)
		if err != nil {
			errs = packer.MultiErrorAppend(
				errs, fmt.Errorf("Error processing %s: %s", n, err))
		}
	}

	sliceTemplates := map[string][]string{
		"inline":           p.config.Inline,
		"scripts":          p.config.Scripts,
		"environment_vars": p.config.Vars,
	}

	for n, slice := range sliceTemplates {
		for i, elem := range slice {
			var err error
			slice[i], err = interpolate.Render(elem, &p.config.ctx)
			if err != nil {
				errs = packer.MultiErrorAppend(
					errs, fmt.Errorf("Error processing %s[%d]: %s", n, i, err))
			}
		}
	}

	if len(p.config.Scripts) == 0 && p.config.Inline == nil {
		errs = packer.MultiErrorAppend(errs,
			errors.New("Either a script file or inline script must be specified."))
	} else if len(p.config.Scripts) > 0 && p.config.Inline != nil {
		errs = packer.MultiErrorAppend(errs,
			errors.New("Only a script file or an inline script can be specified, not both."))
	}

	for _, path := range p.config.Scripts {
		if _, err := os.Stat(path); err != nil {
			errs = packer.MultiErrorAppend(errs,
				fmt.Errorf("Bad script '%s': %s", path, err))
		}
	}

	// Do a check for bad environment variables, such as '=foo', 'foobar'
	for _, kv := range p.config.Vars {
		vs := strings.SplitN(kv, "=", 2)
		if len(vs) != 2 || vs[0] == "" {
			errs = packer.MultiErrorAppend(errs,
				fmt.Errorf("Environment variable not in format 'key=value': %s", kv))
		}
	}

	if errs != nil && len(errs.Errors) > 0 {
		return errs
	}

	return nil
}

func (p *ShellPostProcessor) PostProcess(ui packer.Ui, artifact packer.Artifact) (packer.Artifact, bool, error) {
	keep := p.config.KeepInputArtifact
	scripts := make([]string, len(p.config.Scripts))
	copy(scripts, p.config.Scripts)

	if p.config.Inline != nil {
		tf, err := ioutil.TempFile("", "packer-shell")
		if err != nil {
			return nil, keep, fmt.Errorf("Error preparing shell script: %s", err)
		}
		defer os.Remove(tf.Name())

		// Set the path to the temporary file
		scripts = append(scripts, tf.Name())

		// Write our contents to it
		writer := bufio.NewWriter(tf)
		writer.WriteString(fmt.Sprintf("#!%s\n", p.config.InlineShebang))
		for _, command := range p.config.Inline {
			if _, err := writer.WriteString(command + "\n"); err != nil {
				return nil, keep, fmt.Errorf("Error preparing shell script: %s", err)
			}
		}

		if err := writer.Flush(); err != nil {
			return nil, keep, fmt.Errorf("Error preparing shell script: %s", err)
		}

		tf.Close()
	}

	envVars := make([]string, len(p.config.Vars)+2)
	envVars[0] = "PACKER_BUILD_NAME=" + p.config.PackerBuildName
	envVars[1] = "PACKER_BUILDER_TYPE=" + p.config.PackerBuilderType
	copy(envVars[2:], p.config.Vars)

	files := artifact.Files()
	var stderr bytes.Buffer
	var stdout bytes.Buffer
	fmt.Printf("%+v\n", artifact)
	for _, art := range files {
		for _, path := range scripts {
			stderr.Reset()
			stdout.Reset()
			ui.Say(fmt.Sprintf("Process with shell script: %s", path))

			log.Printf("Opening %s for reading", path)
			f, err := os.Open(path)
			if err != nil {
				return nil, keep, fmt.Errorf("Error opening shell script: %s", err)
			}
			defer f.Close()

			ui.Message(fmt.Sprintf("Executing script with artifact: %s", art))
			args := []string{path, art}
			cmd := exec.Command("/bin/sh", args...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			cmd.Env = envVars
			err = cmd.Run()
			ui.Message(fmt.Sprintf("%s", stdout.String()))
			if err != nil {
				return nil, keep, fmt.Errorf("Unable to execute script: %s", stderr.String())
			}
		}
	}
	return NewArtifact(name, artifact.BuilderId(), outputPath), keep, nil
}
