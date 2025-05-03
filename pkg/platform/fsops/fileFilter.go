package fsops

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var ErrNoDirsProvided = errors.New("attempted to filter but no filter paths were provided")

type FileFilterOption func(*FileFilter) error

// FileFilter operates and filters files over a range of fs.FS objects
type FileFilter struct {
	pattern string
	maxAge  time.Duration
	dir     map[string]fs.FS
	matches []string
	drill   bool
}

func WithGlobPattern(p string) FileFilterOption {
	return func(ff *FileFilter) error {
		// check if the provided pattern is valid
		if _, err := fs.Glob(os.DirFS(""), p); err != nil {
			return err
		}
		ff.pattern = p
		return nil
	}
}

func WithFileAge(d time.Duration) FileFilterOption {
	return func(ff *FileFilter) error {
		ff.maxAge = d
		return nil
	}
}

func SetLoc(loc []string) FileFilterOption {
	return func(ff *FileFilter) error {
		if ff.dir == nil {
			ff.dir = make(map[string]fs.FS)
		}
		for _, location := range loc {
			ff.dir[location] = os.DirFS(location)
		}
		return nil
	}
}

func NewFileFilter(opts ...FileFilterOption) (*FileFilter, error) {
	ff := new(FileFilter)
	for _, opt := range opts {
		err := opt(ff)
		if err != nil {
			return nil, err
		}
	}
	return ff, nil
}

func (ff *FileFilter) SetPattern(p string) error {
	if _, err := fs.Glob(os.DirFS(""), p); err != nil {
		return err
	}
	ff.pattern = p
	return nil
}

// Filter filters the files in the provided directories and returns a list of absolute file paths
func (ff FileFilter) Filter() ([]string, error) {
	if len(ff.dir) == 0 {
		return nil, ErrNoDirsProvided
	}
	// loop over the registered file systems
	for path, fsys := range ff.dir {
		matches, err := fs.Glob(fsys, ff.pattern)
		if err != nil {
			return nil, err
		}
		// if the age filter is set
		if ff.maxAge != 0 {
			for _, m := range matches {
				f, err := fsys.Open(m)
				if err != nil {
					return nil, err
				}

				finfo, _ := f.Stat()
				if finfo.ModTime().After(time.Now().Add(-ff.maxAge)) {
					ff.matches = append(ff.matches, filepath.Join(path, m))
				}
				f.Close()
			}
			continue
		}

		// enrich the found files with the rest of the path stucture before returning
		for idx, m := range matches {
			matches[idx] = filepath.Join(path, m)
		}
		ff.matches = append(ff.matches, matches...)
	}
	return ff.matches, nil
}
