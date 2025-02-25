package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/materia"
)

type MockServices struct {
	Services map[string]string
}

func (mockservices *MockServices) Start(_ context.Context, name string) error {
	if !strings.HasSuffix(name, ".service") {
		name = fmt.Sprintf("%v.service", name)
	}
	if _, ok := mockservices.Services[name]; ok {
		mockservices.Services[name] = "active"
		return nil
	}

	return errors.New("service not found")
}

func (mockservices *MockServices) Stop(_ context.Context, name string) error {
	if !strings.HasSuffix(name, ".service") {
		name = fmt.Sprintf("%v.service", name)
	}
	if _, ok := mockservices.Services[name]; ok {
		mockservices.Services[name] = "stopped"
		return nil
	}
	return errors.New("service not found")
}

func (mockservices *MockServices) Restart(_ context.Context, name string) error {
	if !strings.HasSuffix(name, ".service") {
		name = fmt.Sprintf("%v.service", name)
	}
	if _, ok := mockservices.Services[name]; ok {
		mockservices.Services[name] = "restarted"
		return nil
	}
	return errors.New("service not found")
}

func (mockservices *MockServices) Reload(_ context.Context) error {
	return nil
}

func (mockservices *MockServices) Get(_ context.Context, name string) (*materia.Service, error) {
	if !strings.HasSuffix(name, ".service") {
		name = fmt.Sprintf("%v.service", name)
	}

	if state, ok := mockservices.Services[name]; !ok {
		return nil, errors.New("service not found")
	} else {
		return &materia.Service{
			Name:  name,
			State: state,
		}, nil
	}
}

func (mockservices *MockServices) Close() {
}

type MockContainers struct {
	Volumes map[string]string
}

func (mockcontainers *MockContainers) InspectVolume(name string) (*materia.Volume, error) {
	if mount, ok := mockcontainers.Volumes[name]; !ok {
		return nil, errors.New("volume not found")
	} else {
		return &materia.Volume{
			Name:       name,
			Mountpoint: mount,
		}, nil
	}
}

func (mockcontainers *MockContainers) ListVolumes(_ context.Context) ([]*materia.Volume, error) {
	var vols []*materia.Volume
	for k, v := range mockcontainers.Volumes {
		vols = append(vols, &materia.Volume{
			Name:       k,
			Mountpoint: v,
		})
	}
	return vols, nil
}

func (mc *MockContainers) InstallFile(_ context.Context, _ *materia.Component, _ materia.Resource) error {
	return nil
}

func (mc *MockContainers) RemoveFile(_ context.Context, _ *materia.Component, _ materia.Resource) error {
	return nil
}

func (mockcontainers *MockContainers) Close() {}
