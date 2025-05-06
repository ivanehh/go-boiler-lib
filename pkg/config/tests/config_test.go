package config_test

import (
	"fmt"
	"testing"

	"github.com/ivanehh/go-boiler-lib/pkg/config"
)

func TestLoad(t *testing.T) {
	err := config.Load("/home/terzivan/projects/MDOD/DCS/dcs-gradec/config/cfg.yaml")
	if err != nil {
		t.Fatal(err)
	}
	c := config.Provide()
	fmt.Printf("bc: %+v\n", c.Sources())
}
