package registry

import (
	"strings"
	"testing"

	"github.com/formancehq/fctl/pkg/pluginsdk"
)

func TestCommandHostContract(t *testing.T) {
	cases := map[string]func(*pluginsdk.CommandSpec){
		"reserved flag":  func(c *pluginsdk.CommandSpec) { c.Flags = []pluginsdk.FlagSpec{{Name: "output", Type: "string"}} },
		"reserved child": func(c *pluginsdk.CommandSpec) { c.Subcommands[0].Use = "help" },
		"long shorthand": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "custom", Type: "string", Shorthand: "ab"}}
		},
		"reserved shorthand": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "custom", Type: "string", Shorthand: "o"}}
		},
		"invalid shorthand": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "custom", Type: "string", Shorthand: "1"}}
		},
		"duplicate shorthand": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "one", Type: "string", Shorthand: "x"}, {Name: "two", Type: "string", Shorthand: "x"}}
		},
		"inherited name": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "custom", Type: "string", Persistent: true}}
			c.Subcommands[0].Flags = []pluginsdk.FlagSpec{{Name: "custom", Type: "string"}}
		},
		"inherited shorthand": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "parent", Type: "string", Persistent: true, Shorthand: "x"}}
			c.Subcommands[0].Flags = []pluginsdk.FlagSpec{{Name: "child", Type: "string", Shorthand: "x"}}
		},
		"header whitespace":   func(c *pluginsdk.CommandSpec) { c.Subcommands[0].Use = " info" },
		"header tab":          func(c *pluginsdk.CommandSpec) { c.Subcommands[0].Use = "info\tID" },
		"invalid service":     func(c *pluginsdk.CommandSpec) { c.Service = "../auth" },
		"nonrunnable args":    func(c *pluginsdk.CommandSpec) { c.Args.Max = 1 },
		"nonrunnable confirm": func(c *pluginsdk.CommandSpec) { c.Confirm = true },
		"negative max":        func(c *pluginsdk.CommandSpec) { c.Subcommands[0].Args.Max = -1 },
		"missing confirm":     func(c *pluginsdk.CommandSpec) { c.Subcommands[0].Confirm = true },
		"nonpersistent confirm": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "confirm", Type: "bool", Default: "false"}}
			c.Subcommands[0].Confirm = true
		},
		"unsafe confirm default": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "confirm", Type: "bool", Default: "true"}}
		},
		"multiple bodies": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "one", Type: "string", Body: true}, {Name: "two", Type: "string", Body: true}}
		},
		"inherited body": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "one", Type: "string", Body: true, Persistent: true}}
			c.Subcommands[0].Flags = []pluginsdk.FlagSpec{{Name: "two", Type: "string", Body: true}}
		},
		"body type": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "body", Type: "bool", Default: "false", Body: true}}
		},
		"required true type": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "enabled", Type: "string", RequireTrue: true}}
		},
		"invalid bool": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "enabled", Type: "bool", Default: "yes"}}
		},
		"invalid uint32": func(c *pluginsdk.CommandSpec) {
			c.Flags = []pluginsdk.FlagSpec{{Name: "limit", Type: "uint32", Default: "4294967296"}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := release()
			mutate(&r.Manifest.Root)
			if _, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), "auth", reference()); err == nil {
				t.Fatal("host-incompatible command accepted")
			}
		})
	}
	for root := range coreRoots {
		t.Run("reserved root "+root, func(t *testing.T) {
			r := release()
			r.Service = root
			r.Manifest.Name = root
			r.Manifest.Service = root
			r.Manifest.Root.Use = root
			if _, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), root, reference()); err == nil {
				t.Fatal("reserved root accepted")
			}
		})
	}
}

func TestValidCommandInheritance(t *testing.T) {
	for _, persistent := range []bool{false, true} {
		t.Run(map[bool]string{false: "local", true: "persistent"}[persistent], func(t *testing.T) {
			r := release()
			root := &r.Manifest.Root
			root.Flags = []pluginsdk.FlagSpec{{Name: "parent", Type: "string", Persistent: persistent, Shorthand: "P"}, {Name: "confirm", Type: "bool", Default: "false", Persistent: true}}
			for _, name := range []string{"one", "two"} {
				child := pluginsdk.CommandSpec{Use: name + " ID", Runnable: true, Confirm: true, Args: pluginsdk.ArgsSpec{Min: 1, Max: 1}, Flags: []pluginsdk.FlagSpec{{Name: strings.Repeat("x", 100), Type: "string", Shorthand: "X"}, {Name: "data", Type: "string", Body: true}, {Name: "enabled", Type: "bool", Default: "true", RequireTrue: true}, {Name: "limit", Type: "uint32", Default: "100"}}}
				if !persistent {
					child.Flags = append(child.Flags, pluginsdk.FlagSpec{Name: "parent", Type: "string", Shorthand: "P"})
				}
				root.Subcommands = append(root.Subcommands, child)
			}
			root.Subcommands = append(root.Subcommands, pluginsdk.CommandSpec{Use: strings.Repeat("z", 100), Runnable: true})
			if _, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), "auth", reference()); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestIndexHostBounds(t *testing.T) {
	for _, tc := range []struct {
		name, service, version string
		empty, valid           bool
	}{
		{name: "empty release sequence", service: "auth", version: "2.5.2", empty: true},
		{name: "long service", service: strings.Repeat("a", 65), version: "2.5.2"},
		{name: "max service", service: strings.Repeat("a", 64), version: "2.5.2", valid: true},
		{name: "long version", service: "auth", version: "2.5.2-" + strings.Repeat("a", 123)},
		{name: "max version", service: "auth", version: "2.5.2-" + strings.Repeat("a", 122), valid: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ref := reference()
			ref.ServiceVersion = tc.version
			refs := []Reference{ref}
			if tc.empty {
				refs = []Reference{}
			}
			_, err := ParseIndex(encoded(t, Index{2, map[string]Product{tc.service: {Releases: refs}}}))
			if (err == nil) != tc.valid {
				t.Fatalf("unexpected result: %v", err)
			}
		})
	}
	if _, err := ParseIndex([]byte(`{"schemaVersion":2,"plugins":{}}`)); err != nil {
		t.Fatal(err)
	}
}

func TestValidPersistentBody(t *testing.T) {
	r := release()
	r.Manifest.Root.Flags = []pluginsdk.FlagSpec{{Name: "data", Type: "string", Body: true, Persistent: true}}
	r.Manifest.Root.Subcommands = append(r.Manifest.Root.Subcommands, pluginsdk.CommandSpec{Use: "second", Runnable: true})
	if _, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), "auth", reference()); err != nil {
		t.Fatal(err)
	}
}

func TestAllReservedHostFlags(t *testing.T) {
	for name := range coreFlags {
		t.Run(name, func(t *testing.T) {
			r := release()
			r.Manifest.Root.Subcommands[0].Flags = []pluginsdk.FlagSpec{{Name: name, Type: "string"}}
			if _, err := ParseCatalogue(encoded(t, Catalogue{1, []Release{r}}), "auth", reference()); err == nil {
				t.Fatal("reserved flag accepted")
			}
		})
	}
}
