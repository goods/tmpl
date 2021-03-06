package tmpl

import (
	"bytes"
	"fmt"
	"reflect"
	"strings"
)

//indirect walks up interface/pointer values of a reflect value to get to the
//top level.
func indirect(v reflect.Value) reflect.Value {
	for {
		if v.Kind() == reflect.Interface && !v.IsNil() {
			v = v.Elem()
			continue
		}
		if v.Kind() != reflect.Ptr || v.IsNil() {
			break
		}
		v = v.Elem()
	}
	return v
}

//access attempts to get the map key/struct field from a given reflect value.
func access(stack path, val reflect.Value, key string, set map[string]reflect.Value) (v reflect.Value, err error) {
	pth := stack.StringWith([]string{key})
	//check our path override for that value
	if iv, ex := set[pth]; ex {
		v = iv
		return
	}

	//just go hog wild
	defer func() {
		if e := recover(); e != nil {
			v = reflect.Value{}
			err = fmt.Errorf("%q: %q", pth, e)
		}
	}()

	val = indirect(val)
	switch val.Kind() {
	case reflect.Map:
		v = val.MapIndex(reflect.ValueOf(key))
		if !v.IsValid() {
			err = fmt.Errorf("%q: field not found", pth)
		}
	case reflect.Struct:
		v = val.FieldByName(key)
		if !v.IsValid() {
			err = fmt.Errorf("%q: field not found", pth)
		}
	default:
		err = fmt.Errorf("%q: cant indirect into %q", pth, val.Kind())
	}

	return
}

//context is the type that represents the external information for a template,
//including blocks, functions, and the data structure.
type context struct {
	stack  path
	blocks map[string]*executeBlockValue
	backup map[string]*executeBlockValue
	funcs  map[string]reflect.Value
	set    map[string]reflect.Value
}

//newContext creates a new empty context.
func newContext() *context {
	return &context{
		stack:  path{},
		blocks: map[string]*executeBlockValue{},
		funcs:  map[string]reflect.Value{},
		set:    map[string]reflect.Value{},
	}
}

//String returns a nice pretty represntation of a context.
func (c *context) String() string {
	var buf bytes.Buffer
	fmt.Fprint(&buf, "blocks {")
	for ident, block := range c.blocks {
		fmt.Fprintf(&buf, "\n\t%s: %s", ident, strings.Replace(block.String(), "\n", "\n\t", -1))
	}
	if len(c.blocks) > 0 {
		fmt.Fprint(&buf, "\n")
	}
	fmt.Fprintln(&buf, "}")
	return buf.String()
}

//setFile sets what file the blocks on context were generated from.
func (c *context) setFile(file string) {
	for _, val := range c.blocks {
		val.file = file
	}
}

//dup duplicates all the blocks into the backup value
func (c *context) dup() {
	c.backup = map[string]*executeBlockValue{}
	for key := range c.blocks {
		c.backup[key] = c.blocks[key]
	}
}

//restore copies all the blocks from the backup value into the blocks value
func (c *context) restore() {
	c.blocks = map[string]*executeBlockValue{}
	for key := range c.backup {
		c.blocks[key] = c.backup[key]
	}
}

//valueFor grabs the value for specified selector
func (c *context) valueFor(s *selectorValue) (rv reflect.Value, err error) {
	var pth path
	switch {
	case s == nil:
		err = fmt.Errorf("%q: can't get the value for a nil selector", c.stack)
		return
	case s.abs: //absolute selector starts at top
		pth = path(c.stack[:1])
	case s.pops < 0 || s.pops >= len(c.stack):
		err = fmt.Errorf("%q: cant pop %d items", c.stack, s.pops)
		return
	case s.pops > 0: //relative selector starts pops back
		pth = path(c.stack[:len(c.stack)-(s.pops)])
	default: //start at the top of the stack
		pth = c.stack
	}

	rv, err = pth.valueAt(s.path, c.set)
	return
}

//cd changes the path to the specified selector value
func (c *context) cd(s *selectorValue) (err error) {
	switch {
	case s == nil:
		err = fmt.Errorf("%q: can't get the value for a nil selector", c.stack)
		return
	case s.abs: //absolute selector means start back at the top
		c.stack = c.stack[:1]
	case s.pops < 0 || s.pops >= len(c.stack):
		err = fmt.Errorf("%q: cant pop %d items", c.stack, s.pops)
		return
	case s.pops > 0: //relative selector starts pops back
		c.stack = c.stack[:len(c.stack)-s.pops]
	}
	err = c.stack.cd(s.path, c.set)
	return
}

//setStack sets the path to the specified value.
func (c *context) setStack(p path) {
	c.stack = p
}

//getBlock returns the block with the given name
func (c *context) getBlock(name string) *executeBlockValue {
	return c.blocks[name]
}

//getCall returns the function value with the given name
func (c *context) getCall(name string) reflect.Value {
	return c.funcs[name]
}

//setAt sets a value for the given path, overriding whatever is there
func (c *context) setAt(path string, value interface{}) {
	if path != "" {
		c.set[path] = reflect.ValueOf(value)
	}
}

//unsertAt deletes the value for the given path.
func (c *context) unsetAt(path string) {
	if path != "" {
		delete(c.set, path)
	}
}
