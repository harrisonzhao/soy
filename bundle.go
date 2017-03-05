package soy

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/harrisonzhao/soy/data"
	"github.com/harrisonzhao/soy/parse"
	"github.com/harrisonzhao/soy/parsepasses"
	"github.com/harrisonzhao/soy/soyhtml"
	"github.com/harrisonzhao/soy/template"
)

// Logger is used to print notifications and compile errors when using the
// "WatchFiles" feature.
var Logger = log.New(os.Stderr, "[soy] ", 0)

type soyFile struct{ name, content string }

// Bundle is a collection of soy content (templates and globals).  It acts as
// input for the soy compiler.
type Bundle struct {
	files   []soyFile
	globals data.Map
	err     error
}

// NewBundle returns an empty bundle.
func NewBundle() *Bundle {
	return &Bundle{globals: make(data.Map)}
}

// AddTemplateDir adds all *.soy files found within the given directory
// (including sub-directories) to the bundle.
func (b *Bundle) AddTemplateDir(root string) *Bundle {
	var err = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(path, ".soy") {
			return nil
		}
		b.AddTemplateFile(path)
		return nil
	})
	if err != nil {
		b.err = err
	}
	return b
}

// AddTemplateFile adds the given soy template file text to this bundle.
// If WatchFiles is on, it will be subsequently watched for updates.
func (b *Bundle) AddTemplateFile(filename string) *Bundle {
	content, err := ioutil.ReadFile(filename)
	if err != nil {
		b.err = err
	}
	return b.AddTemplateString(filename, string(content))
}

// AddTemplateString adds the given template to the bundle. The name is only
// used for error messages - it does not need to be provided nor does it need to
// be a real filename.
func (b *Bundle) AddTemplateString(filename, soyfile string) *Bundle {
	b.files = append(b.files, soyFile{filename, soyfile})
	return b
}

// AddGlobalsFile opens and parses the given filename for Soy globals, and adds
// the resulting data map to the bundle.
func (b *Bundle) AddGlobalsFile(filename string) *Bundle {
	var f, err = os.Open(filename)
	if err != nil {
		b.err = err
		return b
	}
	globals, err := ParseGlobals(f)
	if err != nil {
		b.err = err
	}
	f.Close()
	return b.AddGlobalsMap(globals)
}

func (b *Bundle) AddGlobalsMap(globals data.Map) *Bundle {
	for k, v := range globals {
		if existing, ok := b.globals[k]; ok {
			b.err = fmt.Errorf("global %q already defined as %q", k, existing)
			return b
		}
		b.globals[k] = v
	}
	return b
}

// Compile parses all of the soy files in this bundle, verifies a number of
// rules about data references, and returns the completed template registry.
func (b *Bundle) Compile() (*template.Registry, error) {
	if b.err != nil {
		return nil, b.err
	}

	// Compile all the soy (globals are already parsed)
	var registry = template.Registry{}
	for _, soyfile := range b.files {
		var tree, err = parse.SoyFile(soyfile.name, soyfile.content, b.globals)
		if err != nil {
			return nil, err
		}
		if err = registry.Add(tree); err != nil {
			return nil, err
		}
	}

	// Apply the post-parse processing
	var err = parsepasses.CheckDataRefs(registry)
	if err != nil {
		return nil, err
	}

	return &registry, nil
}

// CompileToTofu returns a soyhtml.Tofu object that allows you to render soy
// templates to HTML.
func (b *Bundle) CompileToTofu() (*soyhtml.Tofu, error) {
	var registry, err = b.Compile()
	// TODO: Verify all used funcs exist and have the right # args.
	return soyhtml.NewTofu(registry), err
}
