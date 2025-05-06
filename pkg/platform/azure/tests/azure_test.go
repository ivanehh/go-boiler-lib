package azure_test

import (
	"os"
	"testing"

	"github.com/ivanehh/go-boiler-lib/pkg/config"
	"github.com/ivanehh/go-boiler-lib/pkg/platform/azure"
	"github.com/ivanehh/go-boiler-lib/pkg/platform/logging"
)

// TODO: The test are broken; Rewrite them
func TestNewAzClient(t *testing.T) {
	err := config.Load("/home/terzivan/projects/MDOD/DCS/dcs-gradec/config/cfg.yaml")
	if err != nil {
		t.Fatal(err)
	}
	logging.NewDCSlogger("test", config.Provide().LogConfig())
	acc, err := azure.NewAzContainerClient()
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.Open("/home/terzivan/tmp/Pipfile")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	err = acc.Upload(f, "")
	if err != nil {
		t.Fatal(err)
	}
}
