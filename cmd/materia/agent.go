package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/user"
	"path/filepath"

	"github.com/charmbracelet/log"
)

func defaultSocket() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}
	var socketDir string
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
	socketPath := filepath.Join(socketDir, "materia.sock")

	return socketPath, nil
}

func factsCommand(ctx context.Context, socket string) error {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return err
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Warn("error closing commnad socket", "error", err)
		}
	}()

	cmd := SocketMessage{
		Name: "facts",
	}

	if err := json.NewEncoder(conn).Encode(cmd); err != nil {
		return err
	}

	result, err := io.ReadAll(conn)
	if err != nil {
		return err
	}
	fmt.Println(string(result))
	return nil
}
