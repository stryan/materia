package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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

func factsCommand(_ context.Context, socket string) error {
	resp, err := sendCommand(context.Background(), socket, SocketMessage{Name: "facts"})
	if err != nil {
		return err
	}
	if resp.Name == "error" {
		return fmt.Errorf("server error: %w", errors.New(resp.Data))
	}
	fmt.Println(resp.Data)
	return nil
}

func sendCommand(_ context.Context, socket string, msg SocketMessage) (*SocketMessage, error) {
	conn, err := net.Dial("unix", socket)
	if err != nil {
		return nil, err
	}
	defer func() {
		err := conn.Close()
		if err != nil {
			log.Warn("error closing command socket", "error", err)
		}
	}()

	if err := json.NewEncoder(conn).Encode(msg); err != nil {
		return nil, err
	}

	var resp SocketMessage
	if err := json.NewDecoder(conn).Decode(&resp); err != nil {
		return nil, err
	}
	return &resp, nil
}

func planCommand(_ context.Context, socket string) error {
	resp, err := sendCommand(context.Background(), socket, SocketMessage{Name: "plan"})
	if err != nil {
		return err
	}
	if resp.Name == "error" {
		return fmt.Errorf("server error: %w", errors.New(resp.Data))
	}
	fmt.Println(resp.Data)
	return nil
}

func updateCommand(_ context.Context, socket string) error {
	resp, err := sendCommand(context.Background(), socket, SocketMessage{Name: "update"})
	if err != nil {
		return err
	}
	if resp.Name == "error" {
		return fmt.Errorf("server error: %w", errors.New(resp.Data))
	}
	fmt.Println(resp.Data)
	return nil
}

func syncCommand(_ context.Context, socket string) error {
	resp, err := sendCommand(context.Background(), socket, SocketMessage{Name: "sync"})
	if err != nil {
		return err
	}
	if resp.Name == "error" {
		return fmt.Errorf("server error: %w", errors.New(resp.Data))
	}
	fmt.Println(resp.Data)
	return nil
}
