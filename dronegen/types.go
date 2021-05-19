package main

import (
	"errors"
	"fmt"
	"runtime"
	"strings"
)

// Types to mirror the YAML fields of the drone config.
// See https://docs.drone.io/pipeline/kubernetes/syntax/ and https://docs.drone.io/pipeline/exec/syntax/.

type pipeline struct {
	comment string

	Kind        string           `yaml:"kind"`
	Type        string           `yaml:"type"`
	Name        string           `yaml:"name"`
	Environment map[string]value `yaml:"environment,omitempty"`
	Trigger     trigger          `yaml:"trigger"`
	Workspace   workspace        `yaml:"workspace,omitempty"`
	Platform    platform         `yaml:"platform,omitempty"`
	Clone       clone            `yaml:"clone,omitempty"`
	DependsOn   []string         `yaml:"depends_on,omitempty"`
	Concurrency concurrency      `yaml:"concurrency,omitempty"`
	Steps       []step           `yaml:"steps"`
	Services    []service        `yaml:"services,omitempty"`
	Volumes     []volume         `yaml:"volumes,omitempty"`
}

func newKubePipeline(name string) pipeline {
	return pipeline{
		comment: generatedComment(),
		Kind:    "pipeline",
		Type:    "kubernetes",
		Name:    name,
		Clone:   clone{Disable: true},
	}
}

//nolint:deadcode,unused
func newExecPipeline(name string) pipeline {
	return pipeline{
		comment: generatedComment(),
		Kind:    "pipeline",
		Type:    "exec",
		Name:    name,
		Clone:   clone{Disable: true},
	}
}

func generatedComment() string {
	c := `################################################
# Generated using dronegen, do not edit by hand!
# Use 'make dronegen' to update.
`
	_, file, line, ok := runtime.Caller(2)
	if ok {
		// Trim off the local path to the repo.
		i := strings.LastIndex(file, "dronegen")
		if i > 0 {
			file = file[i:]
		}
		c += fmt.Sprintf("# Generated at %s:%d\n", file, line)
	}
	c += "################################################\n\n"
	return c
}

type trigger struct {
	Event  triggerRef `yaml:"event,omitempty"`
	Cron   triggerRef `yaml:"cron,omitempty"`
	Target triggerRef `yaml:"target,omitempty"`
	Ref    triggerRef `yaml:"ref,omitempty"`
	Repo   triggerRef `yaml:"repo"`
	Branch triggerRef `yaml:"branch,omitempty"`
}

type triggerRef struct {
	Include []string `yaml:"include,omitempty"`
	Exclude []string `yaml:"exclude,omitempty"`
}

// UnmarshalYAML parses trigger references as either a list of strings, or as
// include/exclude lists. Both are allowed by drone.
func (v *triggerRef) UnmarshalYAML(unmarshal func(interface{}) error) error {
	var rawItems []string
	if err := unmarshal(&rawItems); err == nil {
		v.Include = rawItems
		return nil
	}
	var regular struct {
		Include []string `yaml:"include,omitempty"`
		Exclude []string `yaml:"exclude,omitempty"`
	}
	if err := unmarshal(&regular); err == nil {
		v.Include = regular.Include
		v.Exclude = regular.Exclude
		return nil
	}
	return errors.New("can't unmarshal the value as either string or from_secret reference")
}

type workspace struct {
	Path string `yaml:"path"`
}

type clone struct {
	Disable bool `yaml:"disable"`
}

type platform struct {
	OS   string `yaml:"os"`
	Arch string `yaml:"arch"`
}

type concurrency struct {
	Limit int `yaml:"limit"`
}

type volume struct {
	Name  string       `yaml:"name"`
	Temp  *volumeTemp  `yaml:"temp,omitempty"`
	Claim *volumeClaim `yaml:"claim,omitempty"`
}

type volumeTemp struct {
	Medium string `yaml:"medium,omitempty"`
}

type volumeClaim struct {
	Name string `yaml:"name"`
}

type service struct {
	Name    string      `yaml:"name"`
	Image   string      `yaml:"image"`
	Volumes []volumeRef `yaml:"volumes"`
}

type volumeRef struct {
	Name string `yaml:"name"`
	Path string `yaml:"path"`
}

type step struct {
	Name        string           `yaml:"name"`
	Image       string           `yaml:"image,omitempty"`
	Commands    []string         `yaml:"commands,omitempty"`
	Environment map[string]value `yaml:"environment,omitempty"`
	Volumes     []volumeRef      `yaml:"volumes,omitempty"`
	Settings    map[string]value `yaml:"settings,omitempty"`
	Template    []string         `yaml:"template,omitempty"`
	When        *condition       `yaml:"when,omitempty"`
	Failure     string           `yaml:"failure,omitempty"`
}

type condition struct {
	Status []string `yaml:"status,omitempty"`
}

// value is a string value for key:value pairs like "environment" or
// "settings". Values can be either inline strings (raw) or references to
// secrets stored in Drone (fromSecret).
type value struct {
	raw        string
	fromSecret string
}

type valueFromSecret struct {
	FromSecret string `yaml:"from_secret"`
}

func (v value) MarshalYAML() (interface{}, error) {
	if v.raw != "" && v.fromSecret != "" {
		return nil, fmt.Errorf("value %+v has both raw and fromSecret set, can only have one", v)
	}
	if v.raw != "" {
		return v.raw, nil
	}
	if v.fromSecret != "" {
		return valueFromSecret{FromSecret: v.fromSecret}, nil
	}
	return nil, fmt.Errorf("value has neither raw nor fromSecret set, need one")
}

func (v *value) UnmarshalYAML(unmarshal func(interface{}) error) error {
	if err := unmarshal(&v.raw); err == nil {
		return nil
	}
	var fs valueFromSecret
	if err := unmarshal(&fs); err == nil {
		v.fromSecret = fs.FromSecret
		return nil
	}
	return errors.New("can't unmarshal the value as either string or from_secret reference")
}
