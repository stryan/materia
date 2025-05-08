package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"git.saintnet.tech/stryan/materia/internal/containers"
	"git.saintnet.tech/stryan/materia/internal/services"
)

type MockServices struct {
	Services map[string]string
}

func (mockservices *MockServices) Apply(_ context.Context, name string, cmd services.ServiceAction) error {
	if !strings.HasSuffix(name, ".service") && !strings.HasSuffix(name, ".timer") {
		name = fmt.Sprintf("%v.service", name)
	}
	if cmd == services.ServiceReloadUnits {
		return nil
	}
	if _, ok := mockservices.Services[name]; ok {
		state := ""
		switch cmd {
		case services.ServiceRestart:
			state = "active"
		case services.ServiceStart:
			state = "active"
		case services.ServiceStop:
			state = "inactive"
		case services.ServiceEnable, services.ServiceDisable:
		default:
			panic(fmt.Sprintf("unexpected services.ServiceAction: %#v", cmd))
		}
		mockservices.Services[name] = state
		return nil
	}

	return errors.New("service not found")
}

func (mockservices *MockServices) Get(_ context.Context, name string) (*services.Service, error) {
	if !strings.HasSuffix(name, ".service") && !strings.HasSuffix(name, ".timer") {
		name = fmt.Sprintf("%v.service", name)
	}

	if state, ok := mockservices.Services[name]; !ok {
		return nil, services.ErrServiceNotFound
	} else {
		return &services.Service{
			Name:  name,
			State: state,
		}, nil
	}
}

func (ms *MockServices) WaitUntilState(_ context.Context, name string, state string) error {
	if ms.Services[name] == state {
		return nil
	}
	return errors.New("not in state")
}

func (mockservices *MockServices) Close() {
}

type MockContainers struct {
	Volumes map[string]string
}

func (mockcontainers *MockContainers) PauseContainer(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (mockcontainers *MockContainers) UnpauseContainer(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (mockcontainers *MockContainers) DumpVolume(_ context.Context, _ containers.Volume, _ string, _ bool) error {
	panic("not implemented") // TODO: Implement
}

func (mockcontainers *MockContainers) InspectVolume(name string) (*containers.Volume, error) {
	if mount, ok := mockcontainers.Volumes[name]; !ok {
		return nil, errors.New("volume not found")
	} else {
		return &containers.Volume{
			Name:       name,
			Mountpoint: mount,
		}, nil
	}
}

func (mockcontainers *MockContainers) ListVolumes(_ context.Context) ([]*containers.Volume, error) {
	var vols []*containers.Volume
	for k, v := range mockcontainers.Volumes {
		vols = append(vols, &containers.Volume{
			Name:       k,
			Mountpoint: v,
		})
	}
	return vols, nil
}

func (mockcontainers *MockContainers) Close() {}
