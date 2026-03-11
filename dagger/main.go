// Workshop Builder Dagger module — build, test, and publish workshop images.

package main

import (
	"context"
	"dagger/workshop-builder/internal/dagger"
)

type WorkshopBuilder struct{}

func (m *WorkshopBuilder) goBase(src *dagger.Directory) *dagger.Container {
	return dag.Container().
		From("golang:1.24-alpine").
		WithMountedDirectory("/src", src).
		WithWorkdir("/src").
		WithExec([]string{"go", "mod", "tidy"})
}

// Test runs go test ./pkg/workshop/... inside a golang:1.24-alpine container.
func (m *WorkshopBuilder) Test(ctx context.Context, src *dagger.Directory) (string, error) {
	return m.goBase(src).
		WithExec([]string{"go", "test", "./pkg/workshop/...", "-v"}).
		Stdout(ctx)
}

// GoSum returns the generated go.sum content after running go mod tidy.
func (m *WorkshopBuilder) GoSum(ctx context.Context, src *dagger.Directory) (string, error) {
	return m.goBase(src).
		WithExec([]string{"cat", "go.sum"}).
		Stdout(ctx)
}
