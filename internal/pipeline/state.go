package pipeline

import (
	"encoding/json"
	"os"
)

type State struct {
	Steps map[string]bool   `json:"steps"`
	Data  map[string]string `json:"data"`
	path  string
}

func LoadState(path string) (*State, error) {
	s := &State{
		Steps: make(map[string]bool),
		Data:  make(map[string]string),
		path:  path,
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return s, nil
	}

	if err := json.Unmarshal(data, s); err != nil {
		return s, nil
	}

	return s, nil
}

func (s *State) IsCompleted(step string) bool {
	return s.Steps[step]
}

func (s *State) Complete(step string) {
	s.Steps[step] = true
}

func (s *State) Set(key, value string) {
	s.Data[key] = value
}

func (s *State) Get(key string) string {
	return s.Data[key]
}

func (s *State) Reset() {
	s.Steps = make(map[string]bool)
	s.Data = make(map[string]string)
}

func (s *State) Save() error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o644)
}
