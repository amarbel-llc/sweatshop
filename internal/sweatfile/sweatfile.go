package sweatfile

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"

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

func Merge(base, repo Sweatfile) Sweatfile {
	merged := base

	// Arrays: nil = inherit, empty = clear, non-empty = append
	if repo.GitExcludes != nil {
		if len(repo.GitExcludes) == 0 {
			merged.GitExcludes = []string{}
		} else {
			merged.GitExcludes = append(base.GitExcludes, repo.GitExcludes...)
		}
	}
	if repo.Setup != nil {
		if len(repo.Setup) == 0 {
			merged.Setup = []string{}
		} else {
			merged.Setup = append(base.Setup, repo.Setup...)
		}
	}

	// Maps: shallow merge, repo overrides per-key
	if repo.Env != nil {
		if merged.Env == nil {
			merged.Env = make(map[string]string)
		}
		for k, v := range repo.Env {
			merged.Env[k] = v
		}
	}

	if repo.Files != nil {
		if merged.Files == nil {
			merged.Files = make(map[string]FileEntry)
		}
		for k, v := range repo.Files {
			merged.Files[k] = v
		}
	}

	return merged
}

func Save(path string, sf Sweatfile) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(sf)
}

func LoadMerged(engAreaDir, repoDir string) (Sweatfile, error) {
	base, err := Load(filepath.Join(engAreaDir, "sweatfile"))
	if err != nil {
		return Sweatfile{}, err
	}
	repo, err := Load(filepath.Join(repoDir, "sweatfile"))
	if err != nil {
		return Sweatfile{}, err
	}
	return Merge(base, repo), nil
}
