package main

import (
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

func (s *ContainerStore) Create(c *types.ContainerInfo) {
	data, _ := json.Marshal(c)
	path := filepath.Join(s.baseDir, c.ID+".json")
	os.WriteFile(path, data, 0644)
}

func (s *ContainerStore) Get(id string) (*types.ContainerInfo, error) {
	path := filepath.Join(s.baseDir, id+".json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c types.ContainerInfo
	json.Unmarshal(data, &c)
	return &c, nil
}

func (s *ContainerStore) List() []types.ContainerInfo {
	var containers []types.ContainerInfo
	entries, _ := os.ReadDir(s.baseDir)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".json") {
			path := filepath.Join(s.baseDir, e.Name())
			data, _ := os.ReadFile(path)
			var c types.ContainerInfo
			json.Unmarshal(data, &c)
			containers = append(containers, c)
		}
	}
	return containers
}

func (s *ContainerStore) Start(id string) {
	if c, err := s.Get(id); err == nil {
		c.State = types.StatusRunning
		c.Pid = 12345
		s.Create(c)
	}
}

func (s *ContainerStore) Stop(id string) {
	if c, err := s.Get(id); err == nil {
		c.State = types.StatusStopped
		s.Create(c)
	}
}

func (s *ContainerStore) Remove(id string) {
	path := filepath.Join(s.baseDir, id+".json")
	os.Remove(path)
}

func (s *ContainerStore) Logs(id string) string {
	return fmt.Sprintf("[%s] Container logs for %s\n", time.Now().Format(time.RFC3339), id)
}

func generateID() string {
	return fmt.Sprintf("%x", time.Now().UnixNano())
}
