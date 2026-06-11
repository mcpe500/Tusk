package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tusk/tusk/pkg/types"
)

type ContainerStore struct {
	baseDir string
}

func NewContainerStore(baseDir string) *ContainerStore {
	os.MkdirAll(baseDir, 0755)
	return &ContainerStore{baseDir: baseDir}
}

// metaPath returns the path to a container's metadata JSON. Container rootfs
// lives under <baseDir>/<id>/rootfs, so metadata goes in <baseDir>/<id>/meta.json.
func (s *ContainerStore) metaPath(id string) string {
	return filepath.Join(s.baseDir, id, "meta.json")
}

func (s *ContainerStore) Create(c *types.ContainerInfo) {
	if c.Created.IsZero() {
		c.Created = time.Now().UTC()
	}
	data, _ := json.MarshalIndent(c, "", "  ")
	dir := filepath.Join(s.baseDir, c.ID)
	os.MkdirAll(dir, 0755)
	os.WriteFile(s.metaPath(c.ID), data, 0644)
}

func (s *ContainerStore) Get(id string) (*types.ContainerInfo, error) {
	data, err := os.ReadFile(s.metaPath(id))
	if err != nil {
		return nil, err
	}
	var c types.ContainerInfo
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

func (s *ContainerStore) List() []types.ContainerInfo {
	var containers []types.ContainerInfo
	entries, _ := os.ReadDir(s.baseDir)
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		c, err := s.Get(e.Name())
		if err == nil {
			containers = append(containers, *c)
		}
	}
	return containers
}

// FindByNameOrID resolves a container by full ID, 12-char short ID, or name.
func (s *ContainerStore) FindByNameOrID(ref string) (*types.ContainerInfo, error) {
	if c, err := s.Get(ref); err == nil {
		return c, nil
	}
	for _, c := range s.List() {
		if c.Name == ref || strings.HasPrefix(c.ID, ref) {
			cc := c
			return &cc, nil
		}
	}
	return nil, fmt.Errorf("no such container: %s", ref)
}

func (s *ContainerStore) SetState(id string, state types.ContainerStatus, pid int) {
	if c, err := s.Get(id); err == nil {
		c.State = state
		c.Pid = pid
		s.Create(c)
	}
}

func (s *ContainerStore) Remove(id string) {
	os.RemoveAll(filepath.Join(s.baseDir, id))
}

// generateID returns a 32-hex-char unique container ID (timestamp + random).
func generateID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%x%s", time.Now().UnixNano(), hex.EncodeToString(b[:]))
}
