package buildlogger

import "github.com/mongodb/grip/send"

type BuildloggerSender interface {
	send.Sender
	CloseWithExitCode(int) error
}
