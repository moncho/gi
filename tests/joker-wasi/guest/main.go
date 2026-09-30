package main

import (
	"bufio"
	core "github.com/rcarmo/go-joker/core"
	"os"
	"strings"
)

func main() {
	core.Stdout, core.Stderr, core.Stdin = os.Stdout, os.Stderr, os.Stdin
	core.GLOBAL_ENV.InitEnv(core.Stdin, core.Stdout, core.Stderr, nil)
	source := "(println (+ 40 2))"
	if len(os.Args) > 1 {
		source = os.Args[1]
	}
	reader := core.NewReader(bufio.NewReader(strings.NewReader(source)), "<wasm-spike>")
	obj, err := core.TryRead(reader)
	if err != nil {
		panic(err)
	}
	expr, err := core.TryParse(obj, &core.ParseContext{GlobalEnv: core.GLOBAL_ENV})
	if err != nil {
		panic(err)
	}
	if _, err := core.TryEval(expr); err != nil {
		panic(err)
	}
}
