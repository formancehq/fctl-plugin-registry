package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/formancehq/fctl-plugin-registry/internal/registry"
	"io"
	"net/http"
	"os"
	"time"
)

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(args []string, output io.Writer) error {
	flags := flag.NewFlagSet("validate", flag.ContinueOnError)
	flags.SetOutput(io.Discard)
	file := flags.String("file", "registry.yaml", "Central index file")
	offline := flags.Bool("offline", false, "Validate only local index structure (not the promotion gate)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("unexpected positional arguments")
	}
	data, err := os.ReadFile(*file)
	if err != nil {
		return err
	}
	index, err := registry.ParseIndex(data)
	if err != nil {
		return err
	}
	if !*offline {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
		defer cancel()
		v := registry.Validator{Client: &http.Client{Timeout: 2 * time.Minute}}
		if err := v.Validate(ctx, index); err != nil {
			return err
		}
	}
	fmt.Fprintln(output, "Registry validation passed")
	return nil
}
