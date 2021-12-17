package build

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"runtime"

	"github.com/chai2010/ugo/ast"
	"github.com/chai2010/ugo/builtin"
	"github.com/chai2010/ugo/compiler"
	"github.com/chai2010/ugo/lexer"
	"github.com/chai2010/ugo/parser"
	"github.com/chai2010/ugo/token"
)

type Option struct {
	Debug  bool
	GOOS   string
	GOARCH string
	Clang  string
}

type Context struct {
	opt  Option
	path string
	src  string
}

func NewContext(opt *Option) *Context {
	p := &Context{}
	if opt != nil {
		p.opt = *opt
	}
	if p.opt.Clang == "" {
		if runtime.GOOS == "windows" {
			p.opt.Clang, _ = exec.LookPath("clang.exe")
		} else {
			p.opt.Clang, _ = exec.LookPath("clang")
		}
		if p.opt.Clang == "" {
			p.opt.Clang = "clang"
		}
	}
	if p.opt.GOOS == "" {
		p.opt.GOOS = runtime.GOOS
	}
	if p.opt.GOARCH == "" {
		p.opt.GOARCH = runtime.GOARCH
	}

	parser.DebugMode = p.opt.Debug
	return p
}

func (p *Context) Lex(filename string, src interface{}) (tokens, comments []token.Token, err error) {
	code, err := p.readSource(filename, src)
	if err != nil {
		return nil, nil, err
	}

	l := lexer.NewLexer(filename, code)
	tokens = l.Tokens()
	comments = l.Comments()
	return
}

func (p *Context) AST(filename string, src interface{}) (f *ast.File, err error) {
	code, err := p.readSource(filename, src)
	if err != nil {
		return nil, err
	}

	f, err = parser.ParseFile(filename, code)
	if err != nil {
		return nil, err
	}

	return f, nil
}

func (p *Context) ASM(filename string, src interface{}) (ll string, err error) {
	code, err := p.readSource(filename, src)
	if err != nil {
		return "", err
	}

	f, err := parser.ParseFile(filename, code)
	if err != nil {
		return "", err
	}

	ll = new(compiler.Compiler).Compile(f)
	return ll, nil
}

func (p *Context) Build(filename string, src interface{}, outfile string) (output []byte, err error) {
	return p.build(filename, src, outfile, p.opt.GOOS, p.opt.GOARCH)
}

func (p *Context) build(filename string, src interface{}, outfile, goos, goarch string) (output []byte, err error) {
	code, err := p.readSource(filename, src)
	if err != nil {
		return nil, err
	}

	f, err := parser.ParseFile(filename, code)
	if err != nil {
		return nil, err
	}

	const (
		_a_out_ll     = "_a.out.ll"
		_a_builtin_ll = "_a_builtin.out.ll"
	)
	if !p.opt.Debug {
		defer os.Remove(_a_out_ll)
		defer os.Remove(_a_builtin_ll)
	}

	llBuiltin := builtin.GetBuiltinLL(p.opt.GOOS, p.opt.GOARCH)
	err = os.WriteFile(_a_builtin_ll, []byte(llBuiltin), 0666)
	if err != nil {
		return nil, err
	}

	ll := new(compiler.Compiler).Compile(f)
	err = os.WriteFile(_a_out_ll, []byte(ll), 0666)
	if err != nil {
		return nil, err
	}

	if outfile == "" {
		outfile = "a.out"
	}

	cmd := exec.Command(
		p.opt.Clang, "-Wno-override-module", "-o", outfile,
		_a_out_ll, _a_builtin_ll,
	)

	data, err := cmd.CombinedOutput()
	return data, err
}

func (p *Context) Run(filename string, src interface{}) ([]byte, error) {
	a_out := "./a.out"
	if runtime.GOOS == "windows" {
		a_out = `.\a.out.exe`
	}
	if !p.opt.Debug {
		os.Remove(a_out)
	}

	output, err := p.build(filename, src, a_out, runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return output, err
	}

	output, err = exec.Command(a_out).CombinedOutput()
	if err != nil {
		return output, err
	}

	return output, nil
}

func (p *Context) readSource(filename string, src interface{}) (string, error) {
	if src != nil {
		switch s := src.(type) {
		case string:
			return s, nil
		case []byte:
			return string(s), nil
		case *bytes.Buffer:
			if s != nil {
				return s.String(), nil
			}
		case io.Reader:
			d, err := io.ReadAll(s)
			return string(d), err
		}
		return "", errors.New("invalid source")
	}

	d, err := os.ReadFile(filename)
	return string(d), err
}