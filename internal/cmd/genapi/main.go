// Command genapi generates the public API of the pubsub entry-point packages.
//
// The implementation lives in internal/ably, which callers outside this module
// cannot import. Each entry-point package (device, server) has to
// re-export the part of that implementation its users need, which is what this
// command emits.
//
// The set to emit is derived rather than listed: starting from the types a
// package's own constructors hand back, it walks exported fields and methods
// transitively, and then takes every package-level function, constant and
// variable whose signature only mentions types reached that way. A device
// client has no HTTPClient, so nothing reachable only from an HTTPClient — the
// Request and presence-Get options, say — ends up in the device package.
//
// Names the target package already declares by hand are left alone, so the
// hand-written constructors that tag the Ably-Agent header are never
// overwritten by a plain alias to the untagged one underneath.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const (
	modulePath = "github.com/ably/ably-pubsub-go"

	ablyPkg    = "github.com/ably/ably-pubsub-go/internal/ably"
	objectsPkg = "github.com/ably/ably-pubsub-go/internal/ably/objects"

	// objectsPrefix disambiguates the LiveObjects types once they are flattened
	// into a single package: objects.Message and ably.Message would otherwise
	// collide.
	objectsPrefix = "Objects"
)

// target describes one entry-point package to generate for.
type target struct {
	dir string // package directory, relative to the repository root
	pkg string // package name

	// httpOnly drops the part of the API that only an HTTPClient can reach.
	// A device client has no HTTPClient, so its Request and presence-Get
	// options, its channel-status types and its paginated results have no
	// caller there.
	dropHTTPOnly bool

	// deny are ably names never emitted, whatever their reachability. The
	// constructors belong here because the package wraps them to tag the
	// Ably-Agent header, and the entry types because the package names them
	// itself.
	deny []string
}

var targets = map[string]target{
	"device": {
		dir:          "device",
		pkg:          "device",
		dropHTTPOnly: true,
		deny:         []string{"NewRealtime", "NewHTTPClient", "Realtime"},
	},
	"server": {
		dir:  "server",
		pkg:  "server",
		deny: []string{"NewRealtime", "NewHTTPClient", "Realtime", "HTTPClient"},
	},
}

func main() {
	name := flag.String("target", "", "entry-point package to generate for: device or server")
	root := flag.String("root", "", "repository root (default: inferred from the working directory)")
	flag.Parse()

	t, ok := targets[*name]
	if !ok {
		fatalf("unknown -target %q; want device or server", *name)
	}
	repo, err := repoRoot(*root)
	if err != nil {
		fatalf("%v", err)
	}
	if err := run(repo, t); err != nil {
		fatalf("%v", err)
	}
}

func fatalf(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, "genapi: "+format+"\n", args...)
	os.Exit(1)
}

// repoRoot returns the given root, or walks up from the working directory
// looking for the go.mod of this module.
func repoRoot(given string) (string, error) {
	if given != "" {
		return given, nil
	}
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		mod := filepath.Join(dir, "go.mod")
		if b, err := os.ReadFile(mod); err == nil && bytes.Contains(b, []byte("module "+modulePath)) {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod for %s above the working directory", modulePath)
		}
		dir = parent
	}
}

func run(repo string, t target) error {
	out, err := generate(repo, t)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(repo, t.dir, "api_gen.go"), out, 0o644)
}

// generate returns the formatted source of the target's api_gen.go.
func generate(repo string, t target) ([]byte, error) {
	src, err := loadPackage(filepath.Join(repo, "internal", "ably"))
	if err != nil {
		return nil, fmt.Errorf("load internal/ably: %w", err)
	}
	objs, err := loadPackage(filepath.Join(repo, "internal", "ably", "objects"))
	if err != nil {
		return nil, fmt.Errorf("load internal/ably/objects: %w", err)
	}

	declared, err := handWritten(filepath.Join(repo, t.dir))
	if err != nil {
		return nil, fmt.Errorf("scan %s: %w", t.dir, err)
	}

	skip := map[string]bool{}
	for _, n := range t.deny {
		skip[n] = true
	}
	for n := range declared {
		skip[n] = true
	}

	sel := src.selectAPI(t, skip)
	out := render(t, src, objs, sel)

	formatted, err := format.Source(out)
	if err != nil {
		// Keep the unformatted source so the syntax error can be read.
		os.WriteFile(filepath.Join(repo, t.dir, "api_gen.go.broken"), out, 0o644)
		return nil, fmt.Errorf("format generated source (see api_gen.go.broken): %w", err)
	}
	return formatted, nil
}

// pkg is the parsed syntax of one package, indexed by declaration name.
type pkg struct {
	types map[string]*ast.TypeSpec
	// allTypes also holds the unexported types, so that a signature leaking
	// one can be recognised and left out: a wrapper cannot name it, and
	// neither can a caller outside the implementation package.
	allTypes map[string]bool
	typeDoc  map[string]*ast.CommentGroup
	methods  map[string][]*ast.FuncDecl // receiver type name -> exported methods
	funcs    map[string]*ast.FuncDecl
	values   []*valueDecl // consts and vars, in source order
}

// valueDecl is one exported name from a const or var declaration.
type valueDecl struct {
	name  string
	kind  token.Token // token.CONST or token.VAR
	typ   ast.Expr    // declared type, if the spec gave one
	doc   *ast.CommentGroup
	order int
}

func loadPackage(dir string) (*pkg, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go")
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	p := &pkg{
		types:    map[string]*ast.TypeSpec{},
		allTypes: map[string]bool{},
		typeDoc:  map[string]*ast.CommentGroup{},
		methods:  map[string][]*ast.FuncDecl{},
		funcs:    map[string]*ast.FuncDecl{},
	}
	order := 0
	for _, astPkg := range pkgs {
		files := make([]string, 0, len(astPkg.Files))
		for name := range astPkg.Files {
			files = append(files, name)
		}
		sort.Strings(files) // keep generation deterministic across runs
		for _, name := range files {
			p.collect(astPkg.Files[name], &order)
		}
	}
	return p, nil
}

func (p *pkg) collect(f *ast.File, order *int) {
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Recv != nil {
				recv := receiverName(d.Recv)
				if recv != "" && ast.IsExported(d.Name.Name) {
					p.methods[recv] = append(p.methods[recv], d)
				}
				continue
			}
			if ast.IsExported(d.Name.Name) {
				p.funcs[d.Name.Name] = d
			}
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					p.allTypes[s.Name.Name] = true
					if !ast.IsExported(s.Name.Name) {
						continue
					}
					p.types[s.Name.Name] = s
					doc := s.Doc
					if doc == nil {
						doc = d.Doc
					}
					p.typeDoc[s.Name.Name] = doc
				case *ast.ValueSpec:
					if d.Tok != token.CONST && d.Tok != token.VAR {
						continue
					}
					for _, n := range s.Names {
						if !ast.IsExported(n.Name) {
							continue
						}
						doc := s.Doc
						if doc == nil && len(d.Specs) == 1 {
							doc = d.Doc
						}
						p.values = append(p.values, &valueDecl{
							name: n.Name, kind: d.Tok, typ: s.Type, doc: doc, order: *order,
						})
						*order++
					}
				}
			}
		}
	}
}

func receiverName(recv *ast.FieldList) string {
	if len(recv.List) == 0 {
		return ""
	}
	switch t := recv.List[0].Type.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// selection is the set of ably declarations to re-export.
type selection struct {
	types  []string
	funcs  []string
	values []*valueDecl
}

// selectAPI computes the re-export set. Everything exported is in it by
// default, so that nothing is lost by an incomplete walk of the API; a device
// package then drops what only an HTTPClient can reach. Functions and values
// follow their types: one is emitted only if every ably type in its signature
// is itself emitted.
func (p *pkg) selectAPI(t target, skip map[string]bool) selection {
	in := map[string]bool{}
	for name := range p.types {
		in[name] = true
	}
	if t.dropHTTPOnly {
		fromHTTP := p.reachable("HTTPClient")
		fromRealtime := p.reachable("Realtime", "ClientOption", "AuthOption")
		for name := range fromHTTP {
			if !fromRealtime[name] {
				delete(in, name)
			}
		}
	}

	var sel selection
	for name := range in {
		if !skip[name] {
			sel.types = append(sel.types, name)
		}
	}
	sort.Strings(sel.types)

	for name, fn := range p.funcs {
		if skip[name] || !p.withinSet(fn.Type, in) {
			continue
		}
		sel.funcs = append(sel.funcs, name)
	}
	sort.Strings(sel.funcs)

	for _, v := range p.values {
		if skip[v.name] {
			continue
		}
		if v.typ != nil && !p.exprWithinSet(v.typ, in) {
			continue
		}
		sel.values = append(sel.values, v)
	}
	return sel
}

// reachable walks the exported API from the given types, following exported
// fields, method signatures and the parameter types of any function that
// produces something already reached.
func (p *pkg) reachable(seeds ...string) map[string]bool {
	out := map[string]bool{}
	queue := append([]string(nil), seeds...)
	for len(queue) > 0 {
		name := queue[0]
		queue = queue[1:]
		if out[name] {
			continue
		}
		spec, ok := p.types[name]
		if !ok {
			continue
		}
		out[name] = true
		for _, ref := range p.refsOfType(spec) {
			// An unexported type cannot be aliased by name, but the alias of
			// the type mentioning it still works, so it is simply not followed.
			if ast.IsExported(ref) && !out[ref] {
				queue = append(queue, ref)
			}
		}
	}

	// A function that produces a reached type is callable, so the types it
	// takes are reached too even when no method signature mentions them:
	// HistoryWithDirection yields a HistoryOption, which is how Direction gets
	// here. Repeat until the set stops growing.
	for grew := true; grew; {
		grew = false
		for _, fn := range p.funcs {
			results := fields(fn.Type.Results)
			if len(results) == 0 || !p.withinFields(results, out) {
				continue
			}
			for _, f := range fields(fn.Type.Params) {
				for _, ref := range p.localRefs(f.Type) {
					if ast.IsExported(ref) && !out[ref] {
						out[ref] = true
						grew = true
					}
				}
			}
		}
	}
	return out
}

// refsOfType returns the package-local type names a type's exported surface
// mentions: its exported struct fields, the types it is defined in terms of,
// and the signatures of its exported methods.
func (p *pkg) refsOfType(spec *ast.TypeSpec) []string {
	var refs []string
	add := func(e ast.Expr) { refs = append(refs, p.localRefs(e)...) }

	switch t := spec.Type.(type) {
	case *ast.StructType:
		for _, f := range t.Fields.List {
			if len(f.Names) == 0 { // embedded
				add(f.Type)
				continue
			}
			for _, n := range f.Names {
				if ast.IsExported(n.Name) {
					add(f.Type)
					break
				}
			}
		}
	case *ast.InterfaceType:
		for _, m := range t.Methods.List {
			add(m.Type)
		}
	default:
		add(spec.Type)
	}

	for _, m := range p.methods[spec.Name.Name] {
		add(m.Type)
	}
	return refs
}

// localRefs returns the names of package-local types an expression mentions.
// Selector expressions belong to another package, so only their qualifier is
// of interest and it is not a local type.
func (p *pkg) localRefs(e ast.Expr) []string {
	var out []string
	ast.Inspect(e, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.SelectorExpr:
			return false // qualified: another package's type
		case *ast.Ident:
			if p.allTypes[n.Name] {
				out = append(out, n.Name)
			}
		}
		return true
	})
	return out
}

func (p *pkg) withinFields(fs []*ast.Field, set map[string]bool) bool {
	for _, f := range fs {
		if !p.exprWithinSet(f.Type, set) {
			return false
		}
	}
	return true
}

func (p *pkg) withinSet(sig *ast.FuncType, set map[string]bool) bool {
	for _, f := range fields(sig.Params) {
		if !p.exprWithinSet(f.Type, set) {
			return false
		}
	}
	for _, f := range fields(sig.Results) {
		if !p.exprWithinSet(f.Type, set) {
			return false
		}
	}
	return true
}

func (p *pkg) exprWithinSet(e ast.Expr, set map[string]bool) bool {
	for _, ref := range p.localRefs(e) {
		if !ast.IsExported(ref) || !set[ref] {
			return false
		}
	}
	return true
}

func fields(l *ast.FieldList) []*ast.Field {
	if l == nil {
		return nil
	}
	return l.List
}

// handWritten returns the names declared in the target package's own,
// non-generated files.
func handWritten(dir string) (map[string]bool, error) {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return !strings.HasSuffix(fi.Name(), "_test.go") && fi.Name() != "api_gen.go"
	}, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	for _, astPkg := range pkgs {
		for _, f := range astPkg.Files {
			for _, d := range f.Decls {
				switch d := d.(type) {
				case *ast.FuncDecl:
					if d.Recv == nil {
						names[d.Name.Name] = true
					}
				case *ast.GenDecl:
					for _, spec := range d.Specs {
						switch s := spec.(type) {
						case *ast.TypeSpec:
							names[s.Name.Name] = true
						case *ast.ValueSpec:
							for _, n := range s.Names {
								names[n.Name] = true
							}
						}
					}
				}
			}
		}
	}
	return names, nil
}

func render(t target, src, objs *pkg, sel selection) []byte {
	r := &renderer{src: src, objects: objs, imports: map[string]bool{}}

	var body bytes.Buffer
	fmt.Fprintf(&body, "// Types.\n\n")
	for _, name := range sel.types {
		r.doc(&body, src.typeDoc[name])
		fmt.Fprintf(&body, "type %s = ably.%s\n\n", name, name)
	}

	objNames := make([]string, 0, len(objs.types))
	for name := range objs.types {
		objNames = append(objNames, name)
	}
	sort.Strings(objNames)
	if len(objNames) > 0 {
		fmt.Fprintf(&body, "// LiveObjects types. The Objects prefix keeps them apart from the\n"+
			"// same-named types of the main API once both live in one package.\n\n")
		for _, name := range objNames {
			r.doc(&body, objs.typeDoc[name])
			fmt.Fprintf(&body, "type %s%s = objects.%s\n\n", objectsPrefix, name, name)
		}
		r.imports[objectsPkg] = true
	}

	if len(sel.values) > 0 {
		fmt.Fprintf(&body, "// Constants and variables.\n\n")
		for _, v := range sel.values {
			r.doc(&body, v.doc)
			keyword := "const"
			if v.kind == token.VAR {
				keyword = "var"
			}
			fmt.Fprintf(&body, "%s %s = ably.%s\n\n", keyword, v.name, v.name)
		}
	}

	if len(sel.funcs) > 0 {
		fmt.Fprintf(&body, "// Functions.\n\n")
		for _, name := range sel.funcs {
			r.function(&body, src.funcs[name])
		}
	}

	var out bytes.Buffer
	fmt.Fprintf(&out, "// Code generated by internal/cmd/genapi. DO NOT EDIT.\n\n")
	fmt.Fprintf(&out, "package %s\n\n", t.pkg)

	imports := make([]string, 0, len(r.imports)+1)
	imports = append(imports, ablyPkg)
	for path := range r.imports {
		if path != ablyPkg {
			imports = append(imports, path)
		}
	}
	sort.Strings(imports)
	fmt.Fprintf(&out, "import (\n")
	for _, path := range imports {
		if !strings.Contains(path, ".") { // standard library
			fmt.Fprintf(&out, "\t%q\n", path)
		}
	}
	fmt.Fprintf(&out, "\n")
	for _, path := range imports {
		if strings.Contains(path, ".") {
			fmt.Fprintf(&out, "\t%q\n", path)
		}
	}
	fmt.Fprintf(&out, ")\n\n")
	out.Write(body.Bytes())
	return out.Bytes()
}

// ablyQualifier matches the implementation package's qualifier where it
// precedes an exported name, and not where "ably" is part of a URL or of
// ordinary prose.
var ablyQualifier = regexp.MustCompile(`\bably\.([A-Z][A-Za-z0-9_]*)`)

type renderer struct {
	src     *pkg
	objects *pkg
	imports map[string]bool
}

// doc writes a declaration's documentation, rewriting the [ably.X] doc links
// of the implementation package into the [X] of the generated one.
func (r *renderer) doc(w *bytes.Buffer, g *ast.CommentGroup) {
	if g == nil {
		return
	}
	for _, c := range g.List {
		// The implementation package qualifies its own names in doc links and
		// in example snippets; here they are local, and "ably" is not even an
		// import the reader can see.
		text := ablyQualifier.ReplaceAllString(c.Text, "$1")
		fmt.Fprintf(w, "%s\n", text)
	}
}

func (r *renderer) function(w *bytes.Buffer, fn *ast.FuncDecl) {
	r.doc(w, fn.Doc)

	params, args := r.params(fn.Type.Params)
	results := r.results(fn.Type.Results)

	fmt.Fprintf(w, "func %s(%s)", fn.Name.Name, strings.Join(params, ", "))
	switch {
	case len(results) == 1:
		fmt.Fprintf(w, " %s", results[0])
	case len(results) > 1:
		fmt.Fprintf(w, " (%s)", strings.Join(results, ", "))
	}
	fmt.Fprintf(w, " {\n")
	if len(results) > 0 {
		fmt.Fprintf(w, "\treturn ")
	} else {
		fmt.Fprintf(w, "\t")
	}
	fmt.Fprintf(w, "ably.%s(%s)\n}\n\n", fn.Name.Name, strings.Join(args, ", "))
}

// params renders a parameter list, naming anything the source left unnamed,
// and returns the argument list for the forwarding call alongside it.
func (r *renderer) params(l *ast.FieldList) (params, args []string) {
	i := 0
	for _, f := range fields(l) {
		typ := r.expr(f.Type)
		names := make([]string, 0, len(f.Names))
		if len(f.Names) == 0 {
			names = append(names, fmt.Sprintf("a%d", i))
			i++
		}
		for _, n := range f.Names {
			name := n.Name
			if name == "_" || name == "" {
				name = fmt.Sprintf("a%d", i)
			}
			names = append(names, name)
			i++
		}
		params = append(params, fmt.Sprintf("%s %s", strings.Join(names, ", "), typ))
		for _, n := range names {
			if _, variadic := f.Type.(*ast.Ellipsis); variadic {
				n += "..."
			}
			args = append(args, n)
		}
	}
	return params, args
}

func (r *renderer) results(l *ast.FieldList) []string {
	var out []string
	for _, f := range fields(l) {
		typ := r.expr(f.Type)
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		for i := 0; i < n; i++ {
			out = append(out, typ)
		}
	}
	return out
}

// expr renders a type expression for the generated package: package-local
// types keep their bare name because they are aliased here too, objects types
// take the Objects prefix, and anything else keeps its qualifier and has its
// import recorded.
func (r *renderer) expr(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return "*" + r.expr(t.X)
	case *ast.Ellipsis:
		return "..." + r.expr(t.Elt)
	case *ast.ArrayType:
		if t.Len == nil {
			return "[]" + r.expr(t.Elt)
		}
		return "[" + r.expr(t.Len) + "]" + r.expr(t.Elt)
	case *ast.MapType:
		return "map[" + r.expr(t.Key) + "]" + r.expr(t.Value)
	case *ast.ChanType:
		switch t.Dir {
		case ast.SEND:
			return "chan<- " + r.expr(t.Value)
		case ast.RECV:
			return "<-chan " + r.expr(t.Value)
		default:
			return "chan " + r.expr(t.Value)
		}
	case *ast.SelectorExpr:
		qualifier, ok := t.X.(*ast.Ident)
		if !ok {
			return "interface{}"
		}
		if qualifier.Name == "objects" {
			return objectsPrefix + t.Sel.Name
		}
		r.imports[importPath(qualifier.Name)] = true
		return qualifier.Name + "." + t.Sel.Name
	case *ast.FuncType:
		params, _ := r.params(t.Params)
		results := r.results(t.Results)
		s := "func(" + strings.Join(params, ", ") + ")"
		switch {
		case len(results) == 1:
			s += " " + results[0]
		case len(results) > 1:
			s += " (" + strings.Join(results, ", ") + ")"
		}
		return s
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return "interface{}"
		}
	case *ast.BasicLit:
		return t.Value
	}
	return "interface{}"
}

// importPath maps the package names the ably API mentions to their import
// paths. A name that is not listed is a bug in this table rather than
// something to guess at, so it is reported rather than emitted.
var importPaths = map[string]string{
	"context": "context",
	"http":    "net/http",
	"io":      "io",
	"json":    "encoding/json",
	"time":    "time",
	"url":     "net/url",
	"tls":     "crypto/tls",
}

func importPath(name string) string {
	path, ok := importPaths[name]
	if !ok {
		fatalf("no import path known for package %q; add it to importPaths", name)
	}
	return path
}
