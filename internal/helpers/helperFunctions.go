package helpers

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"
)

func Rootpath() (root string) {
	wd, err := os.Getwd()
	if err != nil {
		panic(fmt.Errorf("configuration failed to load:%w", err))
	}
	wdsplit := strings.Split(wd, string(filepath.Separator))
	for idx, pathItem := range wdsplit {
		root = path.Join(root, pathItem)
		if strings.EqualFold(pathItem, "DCS") {
			root = path.Join(root, wdsplit[idx+1])
			if runtime.GOOS != "windows" {
				root = "/" + root
			}
			break
		}
	}
	return root
}
