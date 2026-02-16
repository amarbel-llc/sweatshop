package sweatfile

import (
	"errors"
	"io/fs"
	"os"

	"github.com/BurntSushi/toml"
)

type FileEntry struct {
	Source  string `toml:"source"`
	Content string `toml:"content"`
}

type Sweatfile struct {
	GitExcludes []string             `toml:"git_excludes"`
	Env         map[string]string    `toml:"env"`
	Files       map[string]FileEntry `toml:"files"`
	Setup       []string             `toml:"setup"`
}

func Parse(data []byte) (Sweatfile, error) {
	var sf Sweatfile
	if err := toml.Unmarshal(data, &sf); err != nil {
		return Sweatfile{}, err
	}
	return sf, nil
}

func Load(path string) (Sweatfile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return Sweatfile{}, nil
		}
		return Sweatfile{}, err
	}
	return Parse(data)
}
