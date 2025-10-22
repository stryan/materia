package main

type SocketMessage struct {
	Name string
	Data string
}

func errToMsg(err error) SocketMessage {
	return SocketMessage{
		Name: "error",
		Data: err.Error(),
	}
}
