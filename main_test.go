package main

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexflint/go-arg"
)

func TestAddressCommandsUseEmailFlag(t *testing.T) {
	for _, command := range []string{"renew", "delete", "set-memo"} {
		t.Run(command, func(t *testing.T) {
			for _, flag := range []string{"--email", "--id"} {
				var parsed args
				parser, err := arg.NewParser(arg.Config{}, &parsed)
				if err != nil {
					t.Fatal(err)
				}
				input := []string{"--username", "user@example.com", command, flag, "example@temp.mailbox.org"}
				if command == "set-memo" {
					input = append(input, "--memo", "test memo")
				}
				err = parser.Parse(input)
				if flag == "--email" && err != nil {
					t.Errorf("%s %s: %v", command, flag, err)
				}
				if flag == "--id" && err == nil {
					t.Errorf("%s still accepts obsolete --id flag", command)
				}
				if flag == "--email" && err == nil {
					var help bytes.Buffer
					parser.WriteHelp(&help)
					if !strings.Contains(help.String(), "--email EMAIL") || strings.Contains(help.String(), "--id") {
						t.Errorf("unexpected command help:\n%s", help.String())
					}
				}
			}
		})
	}
}
