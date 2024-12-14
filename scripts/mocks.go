package main

import (
	"context"

	"git.saintnet.tech/stryan/materia/internal/materia"
)

type MockServices struct {
	Services map[string]string
}

func (mockservices *MockServices) Start(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (mockservices *MockServices) Stop(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (mockservices *MockServices) Restart(_ context.Context, _ string) error {
	panic("not implemented") // TODO: Implement
}

func (mockservices *MockServices) Reload(_ context.Context) error {
	panic("not implemented") // TODO: Implement
}

func (mockservices *MockServices) Get(_ context.Context, _ string) (*materia.Service, error) {
	panic("not implemented") // TODO: Implement
}

func (mockservices *MockServices) Close() {
	panic("not implemented") // TODO: Implement
}

type MockContainers struct {
	Volumes map[string]string
}

func (mockcontainers *MockContainers) Inspect(_ string) (*materia.Volume, error) {
	panic("not implemented") // TODO: Implement
}
