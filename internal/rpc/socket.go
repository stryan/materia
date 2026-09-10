package rpc

import (
	"fmt"
	"os"
	"os/user"
	"path/filepath"
)

func socketPath() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	socketDir := ""
	if currentUser.Name != "root" {
		uid := currentUser.Uid
		socketDir = filepath.Join("/run/user", uid, "materia")
	} else {
		socketDir = filepath.Join("/run/materia")
	}
	err = os.MkdirAll(socketDir, 0o700)
	if err != nil {
		return "", err
	}
	// varlink does unix: not the normal scheme://
	return fmt.Sprintf("unix:%v", filepath.Join(socketDir, "materia.sock")), nil
}
