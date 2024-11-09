package javascript

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"regexp"

	"github.com/dop251/goja"

	"github.com/block/scaffolder"
)

type config struct {
	logger func(args ...any)
}

func (o *config) makeLogFunc(prefix string) func(args ...any) {
	return func(args ...any) {
		values := make([]any, len(args)+1)
		values[0] = prefix
		copy(values[1:], args)
		o.logger(values...)
	}
}

// Option is a function that modifies the behaviour of the interpreter.
type Option func(*config)

// WithLogger sets the logger to use for console.log, console.debug, console.error and console.warn.
func WithLogger(logger func(args ...any)) Option {
	return func(o *config) { o.logger = logger }
}

// Extension is a scaffolder extension that allows the use of end-user-provided
// JavaScript code to write template functions.
//
// The extension will execute the JS file scriptPath in the source directory
// if present. If you wish to include a file named scriptPath in the generated
// output, you can name it scriptPath.tmpl.
//
// A global variable named context will be available in the JS VM. It will
// contain the scaffolder.Config.Context value.
//
// Existing template functions will also be available in the JS VM.
func Extension(scriptPath string, options ...Option) scaffolder.Extension {
	conf := &config{
		logger: func(args ...any) { fmt.Fprintln(os.Stderr, args...) },
	}
	for _, option := range options {
		option(conf)
	}
	return scaffolder.ExtensionFunc(func(mutableConfig *scaffolder.Config) error {
		// Exclude the script from the output.
		mutableConfig.Exclude = append(mutableConfig.Exclude, "^"+regexp.QuoteMeta(scriptPath)+"$")

		vm := goja.New()
		vm.SetFieldNameMapper(goja.UncapFieldNameMapper())
		for key, value := range mutableConfig.Funcs {
			if err := vm.Set(key, value); err != nil {
				return err
			}
		}
		if err := initConsole(vm, conf); err != nil {
			return err
		}
		if err := vm.Set("context", mutableConfig.Context); err != nil {
			return err
		}
		scriptPath := filepath.Join(mutableConfig.Source(), scriptPath)
		if script, err := os.ReadFile(scriptPath); err == nil {
			if _, err := vm.RunScript(scriptPath, string(script)); err != nil {
				return fmt.Errorf("failed to run %s: %w", scriptPath, err)
			}
		}

		global := vm.GlobalObject()
		for _, key := range global.Keys() {
			attr := global.Get(key)
			value := attr.Export()
			typ := reflect.TypeOf(value)
			if typ == nil {
				continue
			}
			if typ.Kind() != reflect.Func {
				continue
			}

			// Go functions are exported as is, JS functions are wrapped in a go function that calls them.
			isJsFunc := typ.NumIn() == 1 && typ.In(0) == reflect.TypeOf(goja.FunctionCall{})

			// Go function, expose it directly.
			if !isJsFunc {
				mutableConfig.Funcs[key] = value
				continue
			}

			// JS function, wrap it in func(...any) (any, error)
			fn, ok := goja.AssertFunction(attr)
			if !ok {
				continue
			}
			mutableConfig.Funcs[key] = func(args ...any) (any, error) {
				vmArgs := make([]goja.Value, len(args))
				for i, arg := range args {
					vmArgs[i] = vm.ToValue(arg)
				}
				return fn(global, vmArgs...)
			}
		}
		return nil
	})
}

func initConsole(vm *goja.Runtime, conf *config) error {
	console := vm.NewObject()
	if err := console.Set("log", conf.makeLogFunc("log:")); err != nil {
		return err
	}
	if err := console.Set("debug", conf.makeLogFunc("debug:")); err != nil {
		return err
	}
	if err := console.Set("error", conf.makeLogFunc("error:")); err != nil {
		return err
	}
	if err := console.Set("warn", conf.makeLogFunc("warn:")); err != nil {
		return err
	}
	err := vm.Set("console", console)
	if err != nil {
		return err
	}
	return nil
}
