package main

import (
	"github.com/elastx/elx-pba/cmd"
	"github.com/elastx/elx-pba/cmd/internal/authentication"
	"github.com/elastx/elx-pba/cmd/internal/keyderiviation"
)

func main() {
	cli := cmd.NewCLI(authentication.PasswordAuthenticator{}, keyderiviation.SedutilSha{})
	cli.Start()
}
