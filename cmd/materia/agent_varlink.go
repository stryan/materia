package main

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/varlink/go/varlink"
	"primamateria.systems/materia/pkg/actions"
	varlinkapi "primamateria.systems/materia/pkg/api"
)

type Agent struct {
	socket string
}

func (a *Agent) Facts(ctx context.Context) error {
	conn, err := varlink.NewConnection(ctx, "unix:"+a.socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	facts, err := varlinkapi.Facts().Call(ctx, conn, false)
	if err != nil {
		return err
	}
	fmt.Println(facts)
	return nil
}

func (a *Agent) Plan(ctx context.Context) error {
	conn, err := varlink.NewConnection(ctx, "unix:"+a.socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()

	plan_out, len_out, err := varlinkapi.Plan().Call(ctx, conn)
	if err != nil {
		return err
	}
	if len_out > 0 {
		var actions []actions.Action
		err = json.Unmarshal([]byte(plan_out), &actions)
		if err != nil {
			return err
		}
		fmt.Println("Generated Plan:")
		for _, a := range actions {
			fmt.Println(a.Pretty())
		}
	} else {
		fmt.Println("No changes made")
	}

	return nil
}

func (a *Agent) Sync(ctx context.Context, revision *string) error {
	conn, err := varlink.NewConnection(ctx, "unix:"+a.socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	err = varlinkapi.Sync().Call(ctx, conn, revision)
	if err != nil {
		return err
	}
	return nil
}

func (a *Agent) Update(ctx context.Context) error {
	conn, err := varlink.NewConnection(ctx, "unix:"+a.socket)
	if err != nil {
		return err
	}
	defer func() { _ = conn.Close() }()
	update_out, err := varlinkapi.Update().Call(ctx, conn)
	if err != nil {
		return err
	}
	fmt.Printf("Update ran: %v actions taken", update_out)
	return nil
}
