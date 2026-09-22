// Package buildinfo exposes the Host build version.
package buildinfo

// Version is the Host release, injected at build time with
// -ldflags "-X github.com/guimc233/JustPing/host/internal/buildinfo.Version=<tag>".
// It falls back to "dev" for local builds.
var Version = "dev"
