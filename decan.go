package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/charmbracelet/log"
)

type Decan struct {
	Name      string
	Services  []string
	Resources []Resource
	Enabled   bool
}

func NewDecan(path string) *Decan {
	d := &Decan{Enabled: true}
	d.Name = filepath.Base(path)
	entries, err := os.ReadDir(path)
	if err != nil {
		log.Fatal(err)
	}
	for _, v := range entries {
		newRes := Resource{
			Path:     filepath.Join(path, v.Name()),
			Name:     strings.TrimSuffix(v.Name(), ".gotmpl"),
			Quadlet:  isQuadlet(v.Name()),
			Template: isTemplate(v.Name()),
		}
		d.Resources = append(d.Resources, newRes)
		if newRes.Quadlet && strings.HasSuffix(newRes.Name, ".container") {
			d.Services = append(d.Services, fmt.Sprintf("%v.service", strings.TrimSuffix(newRes.Name, ".container")))
		}
	}
	return d
}

func (d *Decan) ServiceForResource(_ Resource) []string {
	return d.Services
}

func isQuadlet(file string) bool {
	filename := strings.TrimSuffix(file, ".gotmpl")
	quadlets := []string{".pod", ".container", ".network", ".volume", ".kube"}
	return slices.Contains(quadlets, filepath.Ext(filename))
}

func isTemplate(file string) bool {
	return strings.HasSuffix(file, ".gotmpl")
}
